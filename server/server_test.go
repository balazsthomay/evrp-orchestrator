package server_test

import (
	"context"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	longrunningpb "cloud.google.com/go/longrunning/autogen/longrunningpb"
	evrpv1 "github.com/thomaybalazs/evrp-orchestrator/gen/einride/evrp/v1"
	"github.com/thomaybalazs/evrp-orchestrator/orchestrator"
	"github.com/thomaybalazs/evrp-orchestrator/server"
	"github.com/thomaybalazs/evrp-orchestrator/solver"
	"github.com/thomaybalazs/evrp-orchestrator/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const bufSize = 1024 * 1024

func setupServer(t *testing.T) (evrpv1.EVRPServiceClient, longrunningpb.OperationsClient, func()) {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	store := storage.NewMemoryStorage()
	s := solver.NewGreedySolver()
	orch := orchestrator.New(store, s, logger)
	srv := server.New(store, orch, logger)

	lis := bufconn.Listen(bufSize)
	grpcServer := grpc.NewServer()
	evrpv1.RegisterEVRPServiceServer(grpcServer, srv)
	longrunningpb.RegisterOperationsServer(grpcServer, srv)

	go func() {
		_ = grpcServer.Serve(lis)
	}()

	conn, err := grpc.NewClient(
		"passthrough://bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}

	evrpClient := evrpv1.NewEVRPServiceClient(conn)
	opsClient := longrunningpb.NewOperationsClient(conn)

	cleanup := func() {
		conn.Close()
		grpcServer.Stop()
		orch.Shutdown()
	}

	return evrpClient, opsClient, cleanup
}

func testCreateProblemRequest() *evrpv1.CreateProblemRequest {
	now := timestamppb.New(time.Date(2025, 1, 1, 8, 0, 0, 0, time.UTC))
	end := timestamppb.New(time.Date(2025, 1, 1, 20, 0, 0, 0, time.UTC))

	return &evrpv1.CreateProblemRequest{
		Problem: &evrpv1.Problem{
			DisplayName: "Integration Test Problem",
			Shipments: []*evrpv1.Shipment{
				{
					ShipmentId:       "s1",
					PickupLocation:   &evrpv1.Location{Latitude: 59.3293, Longitude: 18.0686},
					DeliveryLocation: &evrpv1.Location{Latitude: 59.3500, Longitude: 18.0200},
					WeightKg:         500,
					DeliveryWindow:   &evrpv1.TimeWindow{StartTime: now, EndTime: end},
					ServiceTimeSeconds: 600,
				},
				{
					ShipmentId:       "s2",
					PickupLocation:   &evrpv1.Location{Latitude: 59.3293, Longitude: 18.0686},
					DeliveryLocation: &evrpv1.Location{Latitude: 59.3100, Longitude: 18.1000},
					WeightKg:         300,
					DeliveryWindow:   &evrpv1.TimeWindow{StartTime: now, EndTime: end},
					ServiceTimeSeconds: 600,
				},
				{
					ShipmentId:       "s3",
					PickupLocation:   &evrpv1.Location{Latitude: 59.3293, Longitude: 18.0686},
					DeliveryLocation: &evrpv1.Location{Latitude: 59.3400, Longitude: 18.0900},
					WeightKg:         400,
					DeliveryWindow:   &evrpv1.TimeWindow{StartTime: now, EndTime: end},
					ServiceTimeSeconds: 600,
				},
			},
			Vehicles: []*evrpv1.Vehicle{
				{
					VehicleId:             "v1",
					BatteryCapacityKwh:    80,
					CurrentChargeKwh:      80,
					EnergyConsumptionRate: 0.2,
					MaxPayloadKg:          2000,
					DepotLocation:         &evrpv1.Location{Latitude: 59.3293, Longitude: 18.0686},
					SpeedKmh:              50,
				},
			},
			Chargers: []*evrpv1.Charger{
				{
					ChargerId: "c1",
					Location:  &evrpv1.Location{Latitude: 59.3200, Longitude: 18.0500},
					NumSlots:  2,
					PowerKw:   150,
				},
			},
			StartTime: now,
			EndTime:   end,
		},
		SolveDuration: durationpb.New(2 * time.Second),
	}
}

func TestCreateProblem_FullFlow(t *testing.T) {
	evrpClient, opsClient, cleanup := setupServer(t)
	defer cleanup()

	ctx := context.Background()

	// Step 1: CreateProblem returns an LRO.
	op, err := evrpClient.CreateProblem(ctx, testCreateProblemRequest())
	if err != nil {
		t.Fatalf("CreateProblem failed: %v", err)
	}

	if op.GetName() == "" {
		t.Fatal("operation name should not be empty")
	}
	if op.GetDone() {
		t.Fatal("operation should not be done immediately")
	}

	t.Logf("created operation: %s", op.GetName())

	// Step 2: Poll the operation until done.
	var finalOp *longrunningpb.Operation
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		finalOp, err = opsClient.GetOperation(ctx, &longrunningpb.GetOperationRequest{
			Name: op.GetName(),
		})
		if err != nil {
			t.Fatalf("GetOperation failed: %v", err)
		}
		if finalOp.GetDone() {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if !finalOp.GetDone() {
		t.Fatal("operation did not complete within timeout")
	}
	if finalOp.GetError() != nil {
		t.Fatalf("operation failed: %v", finalOp.GetError())
	}

	t.Log("operation completed successfully")

	// Step 3: Get the problem and verify state.
	// Extract problem name from operation name: "operations/problems/xxx" -> "problems/xxx"
	problemName := op.GetName()[len("operations/"):]
	problem, err := evrpClient.GetProblem(ctx, &evrpv1.GetProblemRequest{
		Name: problemName,
	})
	if err != nil {
		t.Fatalf("GetProblem failed: %v", err)
	}

	if problem.GetState() != evrpv1.Problem_STATE_SOLVED {
		t.Errorf("expected SOLVED state, got %v", problem.GetState())
	}

	// Step 4: Get the solution.
	solution, err := evrpClient.GetSolution(ctx, &evrpv1.GetSolutionRequest{
		Name: problemName + "/solution",
	})
	if err != nil {
		t.Fatalf("GetSolution failed: %v", err)
	}

	if solution.GetShipmentsAssigned() != 3 {
		t.Errorf("expected 3 shipments assigned, got %d", solution.GetShipmentsAssigned())
	}
	if len(solution.GetRoutes()) == 0 {
		t.Error("expected at least one route")
	}

	t.Logf("solution: %d routes, %.2f km total, %d assigned",
		len(solution.GetRoutes()),
		solution.GetTotalDistanceKm(),
		solution.GetShipmentsAssigned(),
	)
}

func TestListProblems(t *testing.T) {
	evrpClient, _, cleanup := setupServer(t)
	defer cleanup()

	ctx := context.Background()

	// Create 3 problems.
	for i := 0; i < 3; i++ {
		_, err := evrpClient.CreateProblem(ctx, testCreateProblemRequest())
		if err != nil {
			t.Fatalf("CreateProblem %d failed: %v", i, err)
		}
	}

	// List problems.
	resp, err := evrpClient.ListProblems(ctx, &evrpv1.ListProblemsRequest{
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("ListProblems failed: %v", err)
	}

	if len(resp.GetProblems()) != 3 {
		t.Errorf("expected 3 problems, got %d", len(resp.GetProblems()))
	}
}

func TestListProblems_Pagination(t *testing.T) {
	evrpClient, _, cleanup := setupServer(t)
	defer cleanup()

	ctx := context.Background()

	// Create 5 problems.
	for i := 0; i < 5; i++ {
		_, err := evrpClient.CreateProblem(ctx, testCreateProblemRequest())
		if err != nil {
			t.Fatalf("CreateProblem %d failed: %v", i, err)
		}
	}

	// List with page size 2.
	var allProblems []*evrpv1.Problem
	pageToken := ""
	for {
		resp, err := evrpClient.ListProblems(ctx, &evrpv1.ListProblemsRequest{
			PageSize:  2,
			PageToken: pageToken,
		})
		if err != nil {
			t.Fatalf("ListProblems failed: %v", err)
		}
		allProblems = append(allProblems, resp.GetProblems()...)
		if resp.GetNextPageToken() == "" {
			break
		}
		pageToken = resp.GetNextPageToken()
	}

	if len(allProblems) != 5 {
		t.Errorf("expected 5 problems across pages, got %d", len(allProblems))
	}
}

func TestListOperations(t *testing.T) {
	evrpClient, opsClient, cleanup := setupServer(t)
	defer cleanup()

	ctx := context.Background()

	// Create 2 problems (each creates an operation).
	for i := 0; i < 2; i++ {
		_, err := evrpClient.CreateProblem(ctx, testCreateProblemRequest())
		if err != nil {
			t.Fatalf("CreateProblem %d failed: %v", i, err)
		}
	}

	resp, err := opsClient.ListOperations(ctx, &longrunningpb.ListOperationsRequest{
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("ListOperations failed: %v", err)
	}

	if len(resp.GetOperations()) != 2 {
		t.Errorf("expected 2 operations, got %d", len(resp.GetOperations()))
	}
}

func TestGetProblem_NotFound(t *testing.T) {
	evrpClient, _, cleanup := setupServer(t)
	defer cleanup()

	_, err := evrpClient.GetProblem(context.Background(), &evrpv1.GetProblemRequest{
		Name: "problems/nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent problem")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.NotFound {
		t.Errorf("expected NOT_FOUND, got %v", st.Code())
	}
}

func TestGetSolution_NotFound(t *testing.T) {
	evrpClient, _, cleanup := setupServer(t)
	defer cleanup()

	_, err := evrpClient.GetSolution(context.Background(), &evrpv1.GetSolutionRequest{
		Name: "problems/nonexistent/solution",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent solution")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.NotFound {
		t.Errorf("expected NOT_FOUND, got %v", st.Code())
	}
}

func TestGetOperation_NotFound(t *testing.T) {
	_, opsClient, cleanup := setupServer(t)
	defer cleanup()

	_, err := opsClient.GetOperation(context.Background(), &longrunningpb.GetOperationRequest{
		Name: "operations/nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent operation")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.NotFound {
		t.Errorf("expected NOT_FOUND, got %v", st.Code())
	}
}

func TestCreateProblem_InvalidRequest(t *testing.T) {
	evrpClient, _, cleanup := setupServer(t)
	defer cleanup()

	_, err := evrpClient.CreateProblem(context.Background(), &evrpv1.CreateProblemRequest{})
	if err == nil {
		t.Fatal("expected error for empty request")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected INVALID_ARGUMENT, got %v", st.Code())
	}
}

func TestGetOperation_EmptyName(t *testing.T) {
	_, opsClient, cleanup := setupServer(t)
	defer cleanup()

	_, err := opsClient.GetOperation(context.Background(), &longrunningpb.GetOperationRequest{
		Name: "",
	})
	if err == nil {
		t.Fatal("expected error for empty name")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected INVALID_ARGUMENT, got %v", st.Code())
	}
}

func TestListProblems_DefaultPageSize(t *testing.T) {
	evrpClient, _, cleanup := setupServer(t)
	defer cleanup()

	ctx := context.Background()

	// Create one problem.
	_, err := evrpClient.CreateProblem(ctx, testCreateProblemRequest())
	if err != nil {
		t.Fatalf("CreateProblem failed: %v", err)
	}

	// List with default page size (no page_size set).
	resp, err := evrpClient.ListProblems(ctx, &evrpv1.ListProblemsRequest{})
	if err != nil {
		t.Fatalf("ListProblems failed: %v", err)
	}
	if len(resp.GetProblems()) != 1 {
		t.Errorf("expected 1 problem, got %d", len(resp.GetProblems()))
	}
}

func TestListOperations_DefaultPageSize(t *testing.T) {
	evrpClient, opsClient, cleanup := setupServer(t)
	defer cleanup()

	ctx := context.Background()

	_, err := evrpClient.CreateProblem(ctx, testCreateProblemRequest())
	if err != nil {
		t.Fatalf("CreateProblem failed: %v", err)
	}

	resp, err := opsClient.ListOperations(ctx, &longrunningpb.ListOperationsRequest{})
	if err != nil {
		t.Fatalf("ListOperations failed: %v", err)
	}
	if len(resp.GetOperations()) != 1 {
		t.Errorf("expected 1 operation, got %d", len(resp.GetOperations()))
	}
}

func TestListOperations_LargePageSize(t *testing.T) {
	_, opsClient, cleanup := setupServer(t)
	defer cleanup()

	// Page size > 1000 should be clamped.
	resp, err := opsClient.ListOperations(context.Background(), &longrunningpb.ListOperationsRequest{
		PageSize: 5000,
	})
	if err != nil {
		t.Fatalf("ListOperations failed: %v", err)
	}
	if resp.GetOperations() == nil {
		// Empty result is fine, just checking it doesn't error.
		t.Log("empty result as expected")
	}
}

func TestCreateProblem_WithDefaultSolveDuration(t *testing.T) {
	evrpClient, opsClient, cleanup := setupServer(t)
	defer cleanup()

	ctx := context.Background()

	// Create without explicit solve duration.
	req := testCreateProblemRequest()
	req.SolveDuration = nil

	op, err := evrpClient.CreateProblem(ctx, req)
	if err != nil {
		t.Fatalf("CreateProblem failed: %v", err)
	}

	// Wait for completion.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		finalOp, err := opsClient.GetOperation(ctx, &longrunningpb.GetOperationRequest{
			Name: op.GetName(),
		})
		if err != nil {
			t.Fatalf("GetOperation failed: %v", err)
		}
		if finalOp.GetDone() {
			if finalOp.GetError() != nil {
				t.Fatalf("operation failed: %v", finalOp.GetError())
			}
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("operation did not complete within timeout")
}
