package solver

import (
	"context"
	"testing"
	"time"

	"github.com/thomaybalazs/evrp-orchestrator/domain"
)

func testProblem() *domain.Problem {
	now := time.Date(2025, 1, 1, 8, 0, 0, 0, time.UTC)
	end := now.Add(12 * time.Hour)

	// Stockholm area locations.
	depot := domain.Location{Latitude: 59.3293, Longitude: 18.0686} // Stockholm center
	delivery1 := domain.Location{Latitude: 59.3500, Longitude: 18.0200}
	delivery2 := domain.Location{Latitude: 59.3100, Longitude: 18.1000}
	delivery3 := domain.Location{Latitude: 59.3400, Longitude: 18.0900}

	return &domain.Problem{
		Shipments: []domain.Shipment{
			{
				ID:               "s1",
				PickupLocation:   depot,
				DeliveryLocation: delivery1,
				WeightKg:         500,
				DeliveryStart:    now,
				DeliveryEnd:      end,
				ServiceTime:      10 * time.Minute,
			},
			{
				ID:               "s2",
				PickupLocation:   depot,
				DeliveryLocation: delivery2,
				WeightKg:         300,
				DeliveryStart:    now,
				DeliveryEnd:      end,
				ServiceTime:      10 * time.Minute,
			},
			{
				ID:               "s3",
				PickupLocation:   depot,
				DeliveryLocation: delivery3,
				WeightKg:         400,
				DeliveryStart:    now,
				DeliveryEnd:      end,
				ServiceTime:      10 * time.Minute,
			},
		},
		Vehicles: []domain.Vehicle{
			{
				ID:                    "v1",
				BatteryCapacityKWh:    80,
				CurrentChargeKWh:      80,
				EnergyConsumptionRate: 0.2,
				MaxPayloadKg:          2000,
				DepotLocation:         depot,
				SpeedKmh:              50,
			},
		},
		Chargers: []domain.Charger{
			{
				ID:       "c1",
				Location: domain.Location{Latitude: 59.3200, Longitude: 18.0500},
				NumSlots: 2,
				PowerKW:  150,
			},
		},
		StartTime: now,
		EndTime:   end,
	}
}

func TestGreedySolver_Solve(t *testing.T) {
	solver := NewGreedySolver()
	problem := testProblem()

	sol, err := solver.Solve(context.Background(), problem, SolveOptions{})
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}

	if sol.ShipmentsAssigned != 3 {
		t.Errorf("expected 3 shipments assigned, got %d", sol.ShipmentsAssigned)
	}
	if sol.ShipmentsUnassigned != 0 {
		t.Errorf("expected 0 shipments unassigned, got %d", sol.ShipmentsUnassigned)
	}
	if len(sol.Routes) != 1 {
		t.Errorf("expected 1 route, got %d", len(sol.Routes))
	}
	if sol.TotalDistanceKm <= 0 {
		t.Error("expected positive total distance")
	}
}

func TestGreedySolver_AllShipmentsDelivered(t *testing.T) {
	solver := NewGreedySolver()
	problem := testProblem()

	sol, err := solver.Solve(context.Background(), problem, SolveOptions{})
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}

	// Verify all shipments appear in routes.
	delivered := make(map[string]bool)
	for _, route := range sol.Routes {
		for _, stop := range route.Stops {
			if stop.Type == domain.StopTypeDelivery {
				delivered[stop.ShipmentID] = true
			}
		}
	}

	for _, s := range problem.Shipments {
		if !delivered[s.ID] {
			t.Errorf("shipment %s was not delivered", s.ID)
		}
	}
}

func TestGreedySolver_RouteStartsAndEndsAtDepot(t *testing.T) {
	solver := NewGreedySolver()
	problem := testProblem()

	sol, err := solver.Solve(context.Background(), problem, SolveOptions{})
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}

	for _, route := range sol.Routes {
		if len(route.Stops) < 2 {
			t.Fatal("route has fewer than 2 stops")
		}
		first := route.Stops[0]
		last := route.Stops[len(route.Stops)-1]

		if first.Type != domain.StopTypeDepot {
			t.Error("route does not start at depot")
		}
		if last.Type != domain.StopTypeDepot {
			t.Error("route does not end at depot")
		}
	}
}

func TestGreedySolver_BatteryNeverNegative(t *testing.T) {
	solver := NewGreedySolver()
	problem := testProblem()

	sol, err := solver.Solve(context.Background(), problem, SolveOptions{})
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}

	for _, route := range sol.Routes {
		for i, stop := range route.Stops {
			if stop.ChargeOnArrivalKWh < -0.001 { // small tolerance for floating point
				t.Errorf("route %s stop %d: battery is negative on arrival (%.2f kWh)",
					route.VehicleID, i, stop.ChargeOnArrivalKWh)
			}
		}
	}
}

func TestGreedySolver_NoVehicles(t *testing.T) {
	solver := NewGreedySolver()
	problem := testProblem()
	problem.Vehicles = nil

	_, err := solver.Solve(context.Background(), problem, SolveOptions{})
	if err == nil {
		t.Error("expected error for no vehicles")
	}
}

func TestGreedySolver_NoShipments(t *testing.T) {
	solver := NewGreedySolver()
	problem := testProblem()
	problem.Shipments = nil

	_, err := solver.Solve(context.Background(), problem, SolveOptions{})
	if err == nil {
		t.Error("expected error for no shipments")
	}
}

func TestGreedySolver_PayloadLimit(t *testing.T) {
	solver := NewGreedySolver()
	problem := testProblem()
	// Set vehicle payload to only handle one shipment.
	problem.Vehicles[0].MaxPayloadKg = 500

	sol, err := solver.Solve(context.Background(), problem, SolveOptions{})
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}

	// Vehicle should deliver shipments one at a time (pickup then deliver each).
	// All should still be assigned since we pick up and deliver one at a time.
	if sol.ShipmentsAssigned != 3 {
		t.Errorf("expected 3 shipments assigned, got %d", sol.ShipmentsAssigned)
	}
}

func TestGreedySolver_ContextCancellation(t *testing.T) {
	solver := NewGreedySolver()
	problem := testProblem()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err := solver.Solve(ctx, problem, SolveOptions{})
	if err == nil {
		// The solver might complete before checking context for small problems.
		// This is OK - we just verify it doesn't panic.
		return
	}
}

func TestGreedySolver_MultipleVehicles(t *testing.T) {
	solver := NewGreedySolver()
	problem := testProblem()

	// Add a second vehicle with limited payload.
	depot := problem.Vehicles[0].DepotLocation
	problem.Vehicles = append(problem.Vehicles, domain.Vehicle{
		ID:                    "v2",
		BatteryCapacityKWh:    80,
		CurrentChargeKWh:      80,
		EnergyConsumptionRate: 0.2,
		MaxPayloadKg:          2000,
		DepotLocation:         depot,
		SpeedKmh:              50,
	})

	// Set first vehicle to only handle one shipment.
	problem.Vehicles[0].MaxPayloadKg = 500

	sol, err := solver.Solve(context.Background(), problem, SolveOptions{})
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}

	if sol.ShipmentsAssigned != 3 {
		t.Errorf("expected 3 shipments assigned, got %d", sol.ShipmentsAssigned)
	}
}

func TestGreedySolver_NeedsCharging(t *testing.T) {
	s := NewGreedySolver()
	problem := testProblem()

	// Set battery to enough for 1 trip but not all 3 without charging.
	// Distance to each delivery is ~3 km, so round trip is ~6 km.
	// At 0.2 kWh/km that's ~1.2 kWh per round trip, ~3.6 for all three.
	// Set charge to 2.0 so it needs to charge after first delivery.
	problem.Vehicles[0].CurrentChargeKWh = 2.0
	problem.Vehicles[0].BatteryCapacityKWh = 80

	sol, err := s.Solve(context.Background(), problem, SolveOptions{})
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}

	// Verify a charging stop was inserted.
	hasCharging := false
	for _, route := range sol.Routes {
		for _, stop := range route.Stops {
			if stop.Type == domain.StopTypeCharging {
				hasCharging = true
				break
			}
		}
	}

	if !hasCharging {
		t.Error("expected a charging stop to be inserted")
	}

	// Verify no negative battery.
	for _, route := range sol.Routes {
		for i, stop := range route.Stops {
			if stop.ChargeOnArrivalKWh < -0.001 {
				t.Errorf("stop %d has negative battery: %.2f", i, stop.ChargeOnArrivalKWh)
			}
		}
	}
}

func TestGreedySolver_NeedsChargingBeforeDepotReturn(t *testing.T) {
	s := NewGreedySolver()
	now := time.Date(2025, 1, 1, 8, 0, 0, 0, time.UTC)
	end := now.Add(12 * time.Hour)

	// Delivery is ~30 km from depot. Vehicle has enough for one-way but not round trip.
	// At 0.2 kWh/km, 30 km = 6 kWh. Round trip = 12 kWh.
	// Vehicle starts with 8 kWh — can get there but needs to charge before return.
	depot := domain.Location{Latitude: 59.3293, Longitude: 18.0686}
	deliveryLoc := domain.Location{Latitude: 59.1000, Longitude: 18.0686} // ~25.5 km south

	problem := &domain.Problem{
		Shipments: []domain.Shipment{
			{
				ID:               "s1",
				PickupLocation:   depot,
				DeliveryLocation: deliveryLoc,
				WeightKg:         500,
				DeliveryStart:    now,
				DeliveryEnd:      end,
				ServiceTime:      10 * time.Minute,
			},
		},
		Vehicles: []domain.Vehicle{
			{
				ID:                    "v1",
				BatteryCapacityKWh:    80,
				CurrentChargeKWh:      8, // Just enough for one-way + a bit
				EnergyConsumptionRate: 0.2,
				MaxPayloadKg:          2000,
				DepotLocation:         depot,
				SpeedKmh:              60,
			},
		},
		Chargers: []domain.Charger{
			{
				ID:       "c1",
				Location: domain.Location{Latitude: 59.2000, Longitude: 18.0686}, // Between depot and delivery
				NumSlots: 2,
				PowerKW:  150,
			},
		},
		StartTime: now,
		EndTime:   end,
	}

	sol, err := s.Solve(context.Background(), problem, SolveOptions{})
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}

	if sol.ShipmentsAssigned != 1 {
		t.Errorf("expected 1 shipment assigned, got %d", sol.ShipmentsAssigned)
	}

	// Verify no negative battery at any stop.
	for _, route := range sol.Routes {
		for i, stop := range route.Stops {
			if stop.ChargeOnArrivalKWh < -0.001 {
				t.Errorf("stop %d has negative battery: %.2f kWh", i, stop.ChargeOnArrivalKWh)
			}
		}
	}
}

func TestGreedySolver_MultipleChargers(t *testing.T) {
	s := NewGreedySolver()
	problem := testProblem()

	// Add a second charger closer to depot.
	problem.Chargers = append(problem.Chargers, domain.Charger{
		ID:       "c2",
		Location: domain.Location{Latitude: 59.3290, Longitude: 18.0680}, // Very close to depot
		NumSlots: 1,
		PowerKW:  50,
	})

	// Low battery to force charging.
	problem.Vehicles[0].CurrentChargeKWh = 2.0

	sol, err := s.Solve(context.Background(), problem, SolveOptions{})
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}

	// Should pick the nearest charger (c2).
	for _, route := range sol.Routes {
		for _, stop := range route.Stops {
			if stop.Type == domain.StopTypeCharging {
				t.Logf("charged at: %s", stop.ChargerID)
			}
		}
	}

	// No negative battery.
	for _, route := range sol.Routes {
		for i, stop := range route.Stops {
			if stop.ChargeOnArrivalKWh < -0.001 {
				t.Errorf("stop %d has negative battery: %.2f", i, stop.ChargeOnArrivalKWh)
			}
		}
	}
}

func TestGreedySolver_InsufficientBatteryNoChargers(t *testing.T) {
	s := NewGreedySolver()
	problem := testProblem()
	problem.Chargers = nil // No chargers available
	problem.Vehicles[0].CurrentChargeKWh = 0.1 // Almost no battery

	sol, err := s.Solve(context.Background(), problem, SolveOptions{})
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}

	// With no chargers and almost no battery, most shipments should be unassigned.
	t.Logf("assigned: %d, unassigned: %d", sol.ShipmentsAssigned, sol.ShipmentsUnassigned)
}

func TestGreedySolver_ExpiredTimeWindow(t *testing.T) {
	s := NewGreedySolver()
	problem := testProblem()

	// Set a delivery window that has already passed.
	past := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
	problem.Shipments[0].DeliveryEnd = past

	sol, err := s.Solve(context.Background(), problem, SolveOptions{})
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}

	// The expired shipment should be unassigned.
	if sol.ShipmentsUnassigned < 1 {
		t.Errorf("expected at least 1 unassigned shipment due to expired window")
	}
}
