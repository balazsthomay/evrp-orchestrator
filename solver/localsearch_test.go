package solver

import (
	"context"
	"testing"
	"time"

	"github.com/thomaybalazs/evrp-orchestrator/domain"
)

func TestLocalSearchSolver_Solve(t *testing.T) {
	solver := NewLocalSearchSolver()
	problem := testProblem()

	sol, err := solver.Solve(context.Background(), problem, SolveOptions{
		MaxDuration: 500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}

	if sol.ShipmentsAssigned != 3 {
		t.Errorf("expected 3 shipments assigned, got %d", sol.ShipmentsAssigned)
	}
	if sol.TotalDistanceKm <= 0 {
		t.Error("expected positive total distance")
	}
}

func TestLocalSearchSolver_ImprovesOrMaintains(t *testing.T) {
	problem := testProblem()

	greedy := NewGreedySolver()
	greedySol, err := greedy.Solve(context.Background(), problem, SolveOptions{})
	if err != nil {
		t.Fatalf("Greedy solve failed: %v", err)
	}

	ls := NewLocalSearchSolver()
	lsSol, err := ls.Solve(context.Background(), problem, SolveOptions{
		MaxDuration: 1 * time.Second,
	})
	if err != nil {
		t.Fatalf("LocalSearch solve failed: %v", err)
	}

	// Local search should assign at least as many shipments.
	if lsSol.ShipmentsAssigned < greedySol.ShipmentsAssigned {
		t.Errorf("local search assigned fewer shipments: %d vs %d",
			lsSol.ShipmentsAssigned, greedySol.ShipmentsAssigned)
	}

	t.Logf("greedy distance: %.2f km, local search distance: %.2f km",
		greedySol.TotalDistanceKm, lsSol.TotalDistanceKm)
}

func TestLocalSearchSolver_RespectsContext(t *testing.T) {
	solver := NewLocalSearchSolver()
	problem := testProblem()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	sol, err := solver.Solve(ctx, problem, SolveOptions{
		MaxDuration: 10 * time.Second, // Would run long without context.
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}
	if sol == nil {
		t.Fatal("expected non-nil solution")
	}

	// Should finish well before 10 seconds due to context.
	if elapsed > 2*time.Second {
		t.Errorf("solver did not respect context timeout, took %v", elapsed)
	}
}

func TestLocalSearchSolver_BatteryFeasibility(t *testing.T) {
	solver := NewLocalSearchSolver()
	problem := testProblem()

	sol, err := solver.Solve(context.Background(), problem, SolveOptions{
		MaxDuration: 500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}

	for _, route := range sol.Routes {
		var vehicle domain.Vehicle
		for _, v := range problem.Vehicles {
			if v.ID == route.VehicleID {
				vehicle = v
				break
			}
		}

		if err := domain.CheckRouteFeasibility(route, vehicle); err != nil {
			t.Errorf("route %s is not feasible: %v", route.VehicleID, err)
		}
	}
}
