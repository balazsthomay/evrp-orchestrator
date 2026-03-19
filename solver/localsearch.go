package solver

import (
	"context"
	"math"
	"math/rand"
	"time"

	"github.com/thomaybalazs/evrp-orchestrator/domain"
)

// LocalSearchSolver improves a greedy solution using 2-opt with simulated annealing.
type LocalSearchSolver struct {
	greedy *GreedySolver
}

// NewLocalSearchSolver creates a new LocalSearchSolver.
func NewLocalSearchSolver() *LocalSearchSolver {
	return &LocalSearchSolver{
		greedy: NewGreedySolver(),
	}
}

// Solve first constructs a greedy solution, then improves it with local search.
func (s *LocalSearchSolver) Solve(ctx context.Context, problem *domain.Problem, opts SolveOptions) (*domain.Solution, error) {
	// Get initial solution from greedy.
	initial, err := s.greedy.Solve(ctx, problem, opts)
	if err != nil {
		return nil, err
	}

	maxDuration := opts.MaxDuration
	if maxDuration <= 0 {
		maxDuration = 5 * time.Second
	}

	deadline := time.Now().Add(maxDuration)
	best := initial
	bestCost := solutionCost(best)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	temperature := 100.0
	coolingRate := 0.995

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return best, nil
		default:
		}

		// Try a 2-opt swap on a random route.
		candidate := s.twoOptSwap(best, problem, rng)
		if candidate == nil {
			continue
		}

		candidateCost := solutionCost(candidate)

		// Accept if better, or with probability based on temperature.
		delta := candidateCost - bestCost
		if delta < 0 || rng.Float64() < math.Exp(-delta/temperature) {
			best = candidate
			bestCost = candidateCost
		}

		temperature *= coolingRate
		if temperature < 0.01 {
			temperature = 0.01
		}
	}

	return best, nil
}

// twoOptSwap tries to improve a solution by reversing a segment within a route.
func (s *LocalSearchSolver) twoOptSwap(sol *domain.Solution, problem *domain.Problem, rng *rand.Rand) *domain.Solution {
	if len(sol.Routes) == 0 {
		return nil
	}

	// Pick a random route.
	routeIdx := rng.Intn(len(sol.Routes))
	route := sol.Routes[routeIdx]

	// Need at least 4 stops (depot + 2 delivery pairs + depot) to do a swap.
	// Stops include depot-start, pickup/delivery pairs, depot-end.
	// We only swap non-depot stops.
	if len(route.Stops) < 4 {
		return nil
	}

	// Find swappable delivery stop indices (exclude first/last depot and charging stops).
	var swappable []int
	for i := 1; i < len(route.Stops)-1; i++ {
		if route.Stops[i].Type == domain.StopTypePickup || route.Stops[i].Type == domain.StopTypeDelivery {
			swappable = append(swappable, i)
		}
	}

	if len(swappable) < 2 {
		return nil
	}

	// Pick two random positions and try swapping.
	aIdx := rng.Intn(len(swappable))
	bIdx := rng.Intn(len(swappable))
	if aIdx == bIdx {
		return nil
	}
	if aIdx > bIdx {
		aIdx, bIdx = bIdx, aIdx
	}

	i := swappable[aIdx]
	j := swappable[bIdx]

	// Create new stops with the segment reversed.
	newStops := make([]domain.Stop, len(route.Stops))
	copy(newStops, route.Stops)
	newStops[i], newStops[j] = newStops[j], newStops[i]

	// Find the vehicle for this route.
	var vehicle domain.Vehicle
	for _, v := range problem.Vehicles {
		if v.ID == route.VehicleID {
			vehicle = v
			break
		}
	}

	// Recalculate route metrics.
	newRoute := recalculateRoute(newStops, vehicle, problem.StartTime)

	// Check feasibility.
	if err := domain.CheckRouteFeasibility(newRoute, vehicle); err != nil {
		return nil
	}

	// Build new solution.
	newRoutes := make([]domain.Route, len(sol.Routes))
	copy(newRoutes, sol.Routes)
	newRoutes[routeIdx] = newRoute

	var totalDist float64
	var totalDuration int64
	for _, r := range newRoutes {
		totalDist += r.TotalDistanceKm
		totalDuration += r.TotalDurationSeconds
	}

	return &domain.Solution{
		Routes:               newRoutes,
		TotalDistanceKm:      totalDist,
		TotalDurationSeconds: totalDuration,
		ShipmentsAssigned:    sol.ShipmentsAssigned,
		ShipmentsUnassigned:  sol.ShipmentsUnassigned,
	}
}

// recalculateRoute recalculates arrival/departure times, charge levels, and distances.
func recalculateRoute(stops []domain.Stop, vehicle domain.Vehicle, startTime time.Time) domain.Route {
	speed := vehicle.SpeedKmh
	if speed <= 0 {
		speed = 60
	}

	var totalDist float64
	var totalEnergy float64
	currentTime := startTime
	currentCharge := vehicle.CurrentChargeKWh

	for i := range stops {
		if i == 0 {
			stops[i].ArrivalTime = currentTime
			stops[i].DepartureTime = currentTime
			stops[i].ChargeOnArrivalKWh = currentCharge
			stops[i].ChargeOnDepartureKWh = currentCharge
			continue
		}

		dist := domain.HaversineDistance(stops[i-1].Location, stops[i].Location)
		energy := dist * vehicle.EnergyConsumptionRate
		travelTime := domain.TravelTime(stops[i-1].Location, stops[i].Location, speed)

		totalDist += dist
		totalEnergy += energy
		currentCharge -= energy

		arrivalTime := currentTime.Add(travelTime)
		stops[i].ArrivalTime = arrivalTime
		stops[i].ChargeOnArrivalKWh = currentCharge

		// For charging stops, simulate charging.
		if stops[i].Type == domain.StopTypeCharging {
			// Charge to full battery.
			chargeNeeded := vehicle.BatteryCapacityKWh - currentCharge
			if chargeNeeded > 0 {
				// Estimate charging power (use a default if we don't have it).
				chargeDuration := domain.ChargingTime(chargeNeeded, 150) // 150 kW default
				currentTime = arrivalTime.Add(chargeDuration)
				currentCharge = vehicle.BatteryCapacityKWh
			} else {
				currentTime = arrivalTime
			}
		} else {
			// Service time approximation: use existing departure-arrival gap or 5 minutes.
			serviceTime := stops[i].DepartureTime.Sub(stops[i].ArrivalTime)
			if serviceTime <= 0 {
				serviceTime = 5 * time.Minute
			}
			currentTime = arrivalTime.Add(serviceTime)
		}

		stops[i].DepartureTime = currentTime
		stops[i].ChargeOnDepartureKWh = currentCharge
	}

	totalDuration := currentTime.Sub(startTime).Seconds()

	return domain.Route{
		VehicleID:            vehicle.ID,
		Stops:                stops,
		TotalDistanceKm:      totalDist,
		TotalDurationSeconds: int64(totalDuration),
		TotalEnergyKWh:       totalEnergy,
	}
}

// solutionCost returns a cost metric for the solution (lower is better).
func solutionCost(sol *domain.Solution) float64 {
	// Primary: minimize unassigned shipments, secondary: minimize total distance.
	return float64(sol.ShipmentsUnassigned)*1e6 + sol.TotalDistanceKm
}
