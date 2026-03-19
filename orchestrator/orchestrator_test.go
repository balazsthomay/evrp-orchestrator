package orchestrator

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	longrunningpb "cloud.google.com/go/longrunning/autogen/longrunningpb"
	evrpv1 "github.com/thomaybalazs/evrp-orchestrator/gen/einride/evrp/v1"
	"github.com/thomaybalazs/evrp-orchestrator/solver"
	"github.com/thomaybalazs/evrp-orchestrator/storage"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func newTestOrchestrator() *Orchestrator {
	store := storage.NewMemoryStorage()
	s := solver.NewGreedySolver()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	return New(store, s, logger)
}

func testProblem() *evrpv1.Problem {
	now := timestamppb.New(time.Date(2025, 1, 1, 8, 0, 0, 0, time.UTC))
	end := timestamppb.New(time.Date(2025, 1, 1, 20, 0, 0, 0, time.UTC))

	return &evrpv1.Problem{
		Name:        "problems/test-1",
		DisplayName: "Test Problem",
		Shipments: []*evrpv1.Shipment{
			{
				ShipmentId:       "s1",
				PickupLocation:   &evrpv1.Location{Latitude: 59.3293, Longitude: 18.0686},
				DeliveryLocation: &evrpv1.Location{Latitude: 59.3500, Longitude: 18.0200},
				WeightKg:         500,
				DeliveryWindow: &evrpv1.TimeWindow{
					StartTime: now,
					EndTime:   end,
				},
				ServiceTimeSeconds: 600,
			},
			{
				ShipmentId:       "s2",
				PickupLocation:   &evrpv1.Location{Latitude: 59.3293, Longitude: 18.0686},
				DeliveryLocation: &evrpv1.Location{Latitude: 59.3100, Longitude: 18.1000},
				WeightKg:         300,
				DeliveryWindow: &evrpv1.TimeWindow{
					StartTime: now,
					EndTime:   end,
				},
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
	}
}

func TestOrchestrator_SubmitAndComplete(t *testing.T) {
	orch := newTestOrchestrator()
	defer orch.Shutdown()

	problem := testProblem()
	op, err := orch.SubmitProblem(context.Background(), problem, 1*time.Second)
	if err != nil {
		t.Fatalf("SubmitProblem failed: %v", err)
	}

	if op.GetName() == "" {
		t.Error("operation name should not be empty")
	}
	if op.GetDone() {
		t.Error("operation should not be done immediately")
	}

	// Poll for completion.
	var finalOp *longrunningpb.Operation
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		finalOp, err = orch.store.GetOperation(context.Background(), op.GetName())
		if err != nil {
			t.Fatalf("GetOperation failed: %v", err)
		}
		if finalOp.GetDone() {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if finalOp == nil || !finalOp.GetDone() {
		t.Fatal("operation did not complete within timeout")
	}

	// Verify it has a response, not an error.
	if finalOp.GetError() != nil {
		t.Fatalf("operation failed with error: %v", finalOp.GetError())
	}
	if finalOp.GetResponse() == nil {
		t.Fatal("operation has no response")
	}

	// Verify problem state is SOLVED.
	prob, err := orch.store.GetProblem(context.Background(), problem.GetName())
	if err != nil {
		t.Fatalf("GetProblem failed: %v", err)
	}
	if prob.GetState() != evrpv1.Problem_STATE_SOLVED {
		t.Errorf("expected problem state SOLVED, got %v", prob.GetState())
	}

	// Verify solution exists.
	sol, err := orch.store.GetSolution(context.Background(), problem.GetName()+"/solution")
	if err != nil {
		t.Fatalf("GetSolution failed: %v", err)
	}
	if sol.GetShipmentsAssigned() != 2 {
		t.Errorf("expected 2 shipments assigned, got %d", sol.GetShipmentsAssigned())
	}
}

func TestOrchestrator_MultipleProblems(t *testing.T) {
	orch := newTestOrchestrator()
	defer orch.Shutdown()

	// Submit two problems concurrently.
	p1 := testProblem()
	p1.Name = "problems/concurrent-1"

	p2 := testProblem()
	p2.Name = "problems/concurrent-2"

	op1, err := orch.SubmitProblem(context.Background(), p1, 1*time.Second)
	if err != nil {
		t.Fatalf("SubmitProblem 1 failed: %v", err)
	}

	op2, err := orch.SubmitProblem(context.Background(), p2, 1*time.Second)
	if err != nil {
		t.Fatalf("SubmitProblem 2 failed: %v", err)
	}

	// Wait for both to complete.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		o1, _ := orch.store.GetOperation(context.Background(), op1.GetName())
		o2, _ := orch.store.GetOperation(context.Background(), op2.GetName())
		if o1.GetDone() && o2.GetDone() {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Verify both completed successfully.
	for _, opName := range []string{op1.GetName(), op2.GetName()} {
		op, err := orch.store.GetOperation(context.Background(), opName)
		if err != nil {
			t.Fatalf("GetOperation %s failed: %v", opName, err)
		}
		if !op.GetDone() {
			t.Errorf("operation %s did not complete", opName)
		}
		if op.GetError() != nil {
			t.Errorf("operation %s failed: %v", opName, op.GetError())
		}
	}
}

func TestOrchestrator_Shutdown(t *testing.T) {
	orch := newTestOrchestrator()

	problem := testProblem()
	problem.Name = "problems/shutdown-test"
	_, err := orch.SubmitProblem(context.Background(), problem, 30*time.Second) // Long duration
	if err != nil {
		t.Fatalf("SubmitProblem failed: %v", err)
	}

	// Shutdown should cancel the worker.
	orch.Shutdown()

	// Give some time for the goroutine to clean up.
	time.Sleep(200 * time.Millisecond)

	orch.mu.Lock()
	remaining := len(orch.workers)
	orch.mu.Unlock()

	if remaining != 0 {
		t.Errorf("expected 0 workers after shutdown, got %d", remaining)
	}
}

func TestProtoProblemToDomain(t *testing.T) {
	proto := testProblem()
	d := protoProblemToDomain(proto)

	if len(d.Shipments) != 2 {
		t.Errorf("expected 2 shipments, got %d", len(d.Shipments))
	}
	if len(d.Vehicles) != 1 {
		t.Errorf("expected 1 vehicle, got %d", len(d.Vehicles))
	}
	if len(d.Chargers) != 1 {
		t.Errorf("expected 1 charger, got %d", len(d.Chargers))
	}
	if d.Vehicles[0].BatteryCapacityKWh != 80 {
		t.Errorf("expected battery capacity 80, got %f", d.Vehicles[0].BatteryCapacityKWh)
	}
}
