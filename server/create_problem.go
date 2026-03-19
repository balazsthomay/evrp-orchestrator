package server

import (
	"context"

	longrunningpb "cloud.google.com/go/longrunning/autogen/longrunningpb"
	evrpv1 "github.com/thomaybalazs/evrp-orchestrator/gen/einride/evrp/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"buf.build/go/protovalidate"
)

// CreateProblem creates a new eVRP problem and starts solving it.
func (s *Server) CreateProblem(ctx context.Context, req *evrpv1.CreateProblemRequest) (*longrunningpb.Operation, error) {
	// Validate request.
	if err := protovalidate.Validate(req); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
	}

	problem := req.GetProblem()
	if problem == nil {
		return nil, status.Error(codes.InvalidArgument, "problem is required")
	}

	// Generate resource name.
	problem.Name = "problems/" + uuid.New().String()

	// Determine solve duration.
	solveDuration := req.GetSolveDuration().AsDuration()
	if solveDuration <= 0 {
		solveDuration = 5_000_000_000 // 5 seconds default
	}

	s.logger.Info("creating problem",
		"name", problem.GetName(),
		"shipments", len(problem.GetShipments()),
		"vehicles", len(problem.GetVehicles()),
	)

	op, err := s.orchestrator.SubmitProblem(ctx, problem, solveDuration)
	if err != nil {
		s.logger.Error("failed to submit problem", "error", err)
		return nil, status.Errorf(codes.Internal, "failed to submit problem: %v", err)
	}

	return op, nil
}
