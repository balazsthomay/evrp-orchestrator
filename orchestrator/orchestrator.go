package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	longrunningpb "cloud.google.com/go/longrunning/autogen/longrunningpb"
	evrpv1 "github.com/thomaybalazs/evrp-orchestrator/gen/einride/evrp/v1"
	"github.com/thomaybalazs/evrp-orchestrator/solver"
	"github.com/thomaybalazs/evrp-orchestrator/storage"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	rpcstatus "google.golang.org/genproto/googleapis/rpc/status"
)

// Orchestrator manages the lifecycle of eVRP solve operations.
type Orchestrator struct {
	store  storage.Storage
	solver solver.Solver
	logger *slog.Logger

	mu      sync.Mutex
	workers map[string]context.CancelFunc // operation name -> cancel func
}

// New creates a new Orchestrator.
func New(store storage.Storage, solver solver.Solver, logger *slog.Logger) *Orchestrator {
	return &Orchestrator{
		store:   store,
		solver:  solver,
		logger:  logger,
		workers: make(map[string]context.CancelFunc),
	}
}

// SubmitProblem stores the problem, creates an LRO, and dispatches a solver goroutine.
func (o *Orchestrator) SubmitProblem(ctx context.Context, problem *evrpv1.Problem, solveDuration time.Duration) (*longrunningpb.Operation, error) {
	// Set problem metadata.
	problem.CreateTime = timestamppb.Now()
	problem.State = evrpv1.Problem_STATE_SOLVING

	if err := o.store.CreateProblem(ctx, problem); err != nil {
		return nil, fmt.Errorf("store problem: %w", err)
	}

	// Create the LRO.
	opName := fmt.Sprintf("operations/%s", problem.GetName())
	metadata := &evrpv1.SolveMetadata{ProgressPercentage: 0}
	metadataAny, err := anypb.New(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}

	op := &longrunningpb.Operation{
		Name:     opName,
		Metadata: metadataAny,
		Done:     false,
	}

	if err := o.store.CreateOperation(ctx, op); err != nil {
		return nil, fmt.Errorf("store operation: %w", err)
	}

	// Dispatch solver goroutine.
	workerCtx, cancel := context.WithCancel(context.Background())
	o.mu.Lock()
	o.workers[opName] = cancel
	o.mu.Unlock()

	go o.runSolver(workerCtx, problem, opName, solveDuration)

	return proto.Clone(op).(*longrunningpb.Operation), nil
}

// runSolver executes the solver and updates the operation on completion.
func (o *Orchestrator) runSolver(ctx context.Context, problem *evrpv1.Problem, opName string, solveDuration time.Duration) {
	defer func() {
		o.mu.Lock()
		delete(o.workers, opName)
		o.mu.Unlock()
	}()

	o.logger.Info("solver started", "operation", opName, "problem", problem.GetName())

	// Convert proto problem to domain problem.
	domainProblem := protoProblemToDomain(problem)

	opts := solver.SolveOptions{MaxDuration: solveDuration}
	domainSolution, err := o.solver.Solve(ctx, domainProblem, opts)

	if err != nil {
		o.logger.Error("solver failed", "operation", opName, "error", err)
		o.failOperation(opName, problem.GetName(), err)
		return
	}

	// Convert domain solution to proto.
	protoSolution := domainSolutionToProto(domainSolution, problem.GetName())

	// Store solution.
	if err := o.store.CreateSolution(context.Background(), protoSolution); err != nil {
		o.logger.Error("failed to store solution", "operation", opName, "error", err)
		o.failOperation(opName, problem.GetName(), err)
		return
	}

	// Update problem state.
	if err := o.store.UpdateProblemState(context.Background(), problem.GetName(), evrpv1.Problem_STATE_SOLVED); err != nil {
		o.logger.Error("failed to update problem state", "operation", opName, "error", err)
	}

	// Complete the operation.
	responseAny, err := anypb.New(protoSolution)
	if err != nil {
		o.logger.Error("failed to marshal solution", "operation", opName, "error", err)
		return
	}

	op, err := o.store.GetOperation(context.Background(), opName)
	if err != nil {
		o.logger.Error("failed to get operation", "operation", opName, "error", err)
		return
	}

	op.Done = true
	op.Result = &longrunningpb.Operation_Response{Response: responseAny}

	// Update metadata to 100%.
	metadata := &evrpv1.SolveMetadata{ProgressPercentage: 100}
	metadataAny, _ := anypb.New(metadata)
	op.Metadata = metadataAny

	if err := o.store.UpdateOperation(context.Background(), op); err != nil {
		o.logger.Error("failed to update operation", "operation", opName, "error", err)
	}

	o.logger.Info("solver completed",
		"operation", opName,
		"assigned", domainSolution.ShipmentsAssigned,
		"unassigned", domainSolution.ShipmentsUnassigned,
		"distance_km", domainSolution.TotalDistanceKm,
	)
}

// failOperation marks an operation and its problem as failed.
func (o *Orchestrator) failOperation(opName, problemName string, solveErr error) {
	op, err := o.store.GetOperation(context.Background(), opName)
	if err != nil {
		o.logger.Error("failed to get operation for failure update", "operation", opName, "error", err)
		return
	}

	op.Done = true
	op.Result = &longrunningpb.Operation_Error{
		Error: &rpcstatus.Status{
			Code:    13, // INTERNAL
			Message: solveErr.Error(),
		},
	}

	if err := o.store.UpdateOperation(context.Background(), op); err != nil {
		o.logger.Error("failed to update operation with error", "operation", opName, "error", err)
	}

	if err := o.store.UpdateProblemState(context.Background(), problemName, evrpv1.Problem_STATE_FAILED); err != nil {
		o.logger.Error("failed to update problem state to failed", "problem", problemName, "error", err)
	}
}

// Shutdown cancels all running workers.
func (o *Orchestrator) Shutdown() {
	o.mu.Lock()
	defer o.mu.Unlock()

	for name, cancel := range o.workers {
		o.logger.Info("cancelling worker", "operation", name)
		cancel()
	}
}
