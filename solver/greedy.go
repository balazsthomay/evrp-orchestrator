package solver

import (
	"context"
	"fmt"
	"math"

	"github.com/thomaybalazs/evrp-orchestrator/domain"
)

// GreedySolver implements a nearest-neighbor construction heuristic for eVRP.
// It assigns shipments to vehicles using a greedy nearest-neighbor approach,
// inserting charging stops when needed.
type GreedySolver struct{}

// NewGreedySolver creates a new GreedySolver.
func NewGreedySolver() *GreedySolver {
	return &GreedySolver{}
}

// Solve constructs a feasible solution using nearest-neighbor heuristic.
func (s *GreedySolver) Solve(ctx context.Context, problem *domain.Problem, opts SolveOptions) (*domain.Solution, error) {
	if len(problem.Vehicles) == 0 {
		return nil, fmt.Errorf("no vehicles available")
	}
	if len(problem.Shipments) == 0 {
		return nil, fmt.Errorf("no shipments to deliver")
	}

	// Build routes for each vehicle using nearest-neighbor.
	assigned := make(map[int]bool)
	routes := make([]domain.Route, 0, len(problem.Vehicles))

	for _, vehicle := range problem.Vehicles {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		route := s.buildRoute(vehicle, problem, assigned)
		if len(route.Stops) > 2 { // More than just depot start/end
			routes = append(routes, route)
		}
	}

	// Count assignments.
	shipmentsAssigned := len(assigned)
	shipmentsUnassigned := len(problem.Shipments) - shipmentsAssigned

	// If there are unassigned shipments and we didn't use all vehicles,
	// this is as good as the greedy gets.
	var totalDist float64
	var totalDuration int64
	for _, r := range routes {
		totalDist += r.TotalDistanceKm
		totalDuration += r.TotalDurationSeconds
	}

	return &domain.Solution{
		Routes:               routes,
		TotalDistanceKm:      totalDist,
		TotalDurationSeconds: totalDuration,
		ShipmentsAssigned:    shipmentsAssigned,
		ShipmentsUnassigned:  shipmentsUnassigned,
	}, nil
}

// buildRoute constructs a route for a single vehicle using nearest-neighbor.
func (s *GreedySolver) buildRoute(vehicle domain.Vehicle, problem *domain.Problem, assigned map[int]bool) domain.Route {
	speed := vehicle.SpeedKmh
	if speed <= 0 {
		speed = 60
	}

	currentLoc := vehicle.DepotLocation
	currentCharge := vehicle.CurrentChargeKWh
	currentPayload := 0.0
	currentTime := problem.StartTime

	stops := []domain.Stop{
		{
			Type:                 domain.StopTypeDepot,
			Location:             vehicle.DepotLocation,
			ArrivalTime:          currentTime,
			DepartureTime:        currentTime,
			ChargeOnArrivalKWh:   currentCharge,
			ChargeOnDepartureKWh: currentCharge,
		},
	}

	var totalDist float64
	var totalEnergy float64

	for {
		// Find nearest reachable unassigned shipment.
		bestIdx := -1
		bestDist := math.MaxFloat64

		for i, shipment := range problem.Shipments {
			if assigned[i] {
				continue
			}

			// Check if vehicle can carry the shipment.
			if currentPayload+shipment.WeightKg > vehicle.MaxPayloadKg {
				continue
			}

			// Distance: current -> pickup -> delivery -> depot (for feasibility check).
			distToPickup := domain.HaversineDistance(currentLoc, shipment.PickupLocation)
			distPickupToDelivery := domain.HaversineDistance(shipment.PickupLocation, shipment.DeliveryLocation)
			distDeliveryToDepot := domain.HaversineDistance(shipment.DeliveryLocation, vehicle.DepotLocation)

			totalTripDist := distToPickup + distPickupToDelivery + distDeliveryToDepot
			energyNeeded := totalTripDist * vehicle.EnergyConsumptionRate

			// Can we do pickup -> delivery -> depot with current (or chargeable) energy?
			if !s.canCompleteTrip(currentCharge, currentLoc, shipment, vehicle, problem.Chargers) {
				continue
			}

			// Check time window feasibility.
			travelToPickup := domain.TravelTime(currentLoc, shipment.PickupLocation, speed)
			arriveAtPickup := currentTime.Add(travelToPickup)
			departPickup := arriveAtPickup.Add(shipment.ServiceTime)
			travelToDelivery := domain.TravelTime(shipment.PickupLocation, shipment.DeliveryLocation, speed)
			arriveAtDelivery := departPickup.Add(travelToDelivery)

			// Check delivery window.
			if arriveAtDelivery.After(shipment.DeliveryEnd) {
				continue
			}

			_ = energyNeeded // used for feasibility check above

			if distToPickup < bestDist {
				bestDist = distToPickup
				bestIdx = i
			}
		}

		if bestIdx == -1 {
			break // No more reachable shipments.
		}

		shipment := problem.Shipments[bestIdx]

		// Check if we need to charge before pickup.
		distToPickup := domain.HaversineDistance(currentLoc, shipment.PickupLocation)
		energyToPickup := distToPickup * vehicle.EnergyConsumptionRate
		distPickupToDelivery := domain.HaversineDistance(shipment.PickupLocation, shipment.DeliveryLocation)
		energyPickupToDelivery := distPickupToDelivery * vehicle.EnergyConsumptionRate
		distDeliveryToDepot := domain.HaversineDistance(shipment.DeliveryLocation, vehicle.DepotLocation)
		energyDeliveryToDepot := distDeliveryToDepot * vehicle.EnergyConsumptionRate

		totalEnergyNeeded := energyToPickup + energyPickupToDelivery + energyDeliveryToDepot

		if currentCharge < totalEnergyNeeded && len(problem.Chargers) > 0 {
			// Insert a charging stop.
			charger := s.findNearestCharger(currentLoc, problem.Chargers)
			distToCharger := domain.HaversineDistance(currentLoc, charger.Location)
			energyToCharger := distToCharger * vehicle.EnergyConsumptionRate

			if currentCharge >= energyToCharger {
				travelToCharger := domain.TravelTime(currentLoc, charger.Location, speed)
				arriveAtCharger := currentTime.Add(travelToCharger)
				chargeAfterTravel := currentCharge - energyToCharger

				// Charge enough for the remaining trip + some buffer.
				energyNeeded := totalEnergyNeeded - chargeAfterTravel + energyToCharger
				if energyNeeded < 0 {
					energyNeeded = 0
				}
				// Don't exceed battery capacity.
				maxChargeable := vehicle.BatteryCapacityKWh - chargeAfterTravel
				if energyNeeded > maxChargeable {
					energyNeeded = maxChargeable
				}
				chargeDuration := domain.ChargingTime(energyNeeded, charger.PowerKW)
				departCharger := arriveAtCharger.Add(chargeDuration)
				chargeAfterCharging := chargeAfterTravel + energyNeeded

				stops = append(stops, domain.Stop{
					Type:                 domain.StopTypeCharging,
					Location:             charger.Location,
					ChargerID:            charger.ID,
					ArrivalTime:          arriveAtCharger,
					DepartureTime:        departCharger,
					ChargeOnArrivalKWh:   chargeAfterTravel,
					ChargeOnDepartureKWh: chargeAfterCharging,
				})

				totalDist += distToCharger
				totalEnergy += energyToCharger
				currentLoc = charger.Location
				currentCharge = chargeAfterCharging
				currentTime = departCharger

				// Recalculate distances from charger.
				distToPickup = domain.HaversineDistance(currentLoc, shipment.PickupLocation)
				energyToPickup = distToPickup * vehicle.EnergyConsumptionRate
			}
		}

		// Add pickup stop.
		travelToPickup := domain.TravelTime(currentLoc, shipment.PickupLocation, speed)
		arriveAtPickup := currentTime.Add(travelToPickup)
		chargeAtPickup := currentCharge - energyToPickup
		departPickup := arriveAtPickup.Add(shipment.ServiceTime)

		stops = append(stops, domain.Stop{
			Type:                 domain.StopTypePickup,
			Location:             shipment.PickupLocation,
			ShipmentID:           shipment.ID,
			ArrivalTime:          arriveAtPickup,
			DepartureTime:        departPickup,
			ChargeOnArrivalKWh:   chargeAtPickup,
			ChargeOnDepartureKWh: chargeAtPickup,
			PayloadOnDepartureKg: currentPayload + shipment.WeightKg,
		})

		totalDist += distToPickup
		totalEnergy += energyToPickup
		currentLoc = shipment.PickupLocation
		currentCharge = chargeAtPickup
		currentPayload += shipment.WeightKg
		currentTime = departPickup

		// Add delivery stop.
		travelToDelivery := domain.TravelTime(currentLoc, shipment.DeliveryLocation, speed)
		arriveAtDelivery := currentTime.Add(travelToDelivery)
		chargeAtDelivery := currentCharge - energyPickupToDelivery
		departDelivery := arriveAtDelivery.Add(shipment.ServiceTime)

		stops = append(stops, domain.Stop{
			Type:                 domain.StopTypeDelivery,
			Location:             shipment.DeliveryLocation,
			ShipmentID:           shipment.ID,
			ArrivalTime:          arriveAtDelivery,
			DepartureTime:        departDelivery,
			ChargeOnArrivalKWh:   chargeAtDelivery,
			ChargeOnDepartureKWh: chargeAtDelivery,
			PayloadOnDepartureKg: currentPayload - shipment.WeightKg,
		})

		totalDist += distPickupToDelivery
		totalEnergy += energyPickupToDelivery
		currentLoc = shipment.DeliveryLocation
		currentCharge = chargeAtDelivery
		currentPayload -= shipment.WeightKg
		currentTime = departDelivery

		assigned[bestIdx] = true
	}

	// Return to depot.
	distToDepot := domain.HaversineDistance(currentLoc, vehicle.DepotLocation)
	energyToDepot := distToDepot * vehicle.EnergyConsumptionRate

	// Check if we need to charge before returning to depot.
	if currentCharge < energyToDepot && len(problem.Chargers) > 0 {
		charger := s.findNearestCharger(currentLoc, problem.Chargers)
		distToCharger := domain.HaversineDistance(currentLoc, charger.Location)
		energyToCharger := distToCharger * vehicle.EnergyConsumptionRate

		if currentCharge >= energyToCharger {
			travelToCharger := domain.TravelTime(currentLoc, charger.Location, speed)
			arriveAtCharger := currentTime.Add(travelToCharger)
			chargeAfterTravel := currentCharge - energyToCharger

			distChargerToDepot := domain.HaversineDistance(charger.Location, vehicle.DepotLocation)
			energyChargerToDepot := distChargerToDepot * vehicle.EnergyConsumptionRate
			energyNeeded := energyChargerToDepot - chargeAfterTravel
			if energyNeeded < 0 {
				energyNeeded = 0
			}
			maxChargeable := vehicle.BatteryCapacityKWh - chargeAfterTravel
			if energyNeeded > maxChargeable {
				energyNeeded = maxChargeable
			}
			chargeDuration := domain.ChargingTime(energyNeeded, charger.PowerKW)
			departCharger := arriveAtCharger.Add(chargeDuration)
			chargeAfterCharging := chargeAfterTravel + energyNeeded

			stops = append(stops, domain.Stop{
				Type:                 domain.StopTypeCharging,
				Location:             charger.Location,
				ChargerID:            charger.ID,
				ArrivalTime:          arriveAtCharger,
				DepartureTime:        departCharger,
				ChargeOnArrivalKWh:   chargeAfterTravel,
				ChargeOnDepartureKWh: chargeAfterCharging,
			})

			totalDist += distToCharger
			totalEnergy += energyToCharger
			currentLoc = charger.Location
			currentCharge = chargeAfterCharging
			currentTime = departCharger

			distToDepot = distChargerToDepot
			energyToDepot = energyChargerToDepot
		}
	}

	travelToDepot := domain.TravelTime(currentLoc, vehicle.DepotLocation, speed)
	arriveAtDepot := currentTime.Add(travelToDepot)
	chargeAtDepot := currentCharge - energyToDepot

	stops = append(stops, domain.Stop{
		Type:                 domain.StopTypeDepot,
		Location:             vehicle.DepotLocation,
		ArrivalTime:          arriveAtDepot,
		DepartureTime:        arriveAtDepot,
		ChargeOnArrivalKWh:   chargeAtDepot,
		ChargeOnDepartureKWh: chargeAtDepot,
	})

	totalDist += distToDepot
	totalEnergy += energyToDepot

	totalDuration := arriveAtDepot.Sub(problem.StartTime).Seconds()

	return domain.Route{
		VehicleID:            vehicle.ID,
		Stops:                stops,
		TotalDistanceKm:      totalDist,
		TotalDurationSeconds: int64(totalDuration),
		TotalEnergyKWh:       totalEnergy,
	}
}

// canCompleteTrip checks if a vehicle can do current -> pickup -> delivery -> depot
// possibly with a charging stop.
func (s *GreedySolver) canCompleteTrip(currentCharge float64, currentLoc domain.Location, shipment domain.Shipment, vehicle domain.Vehicle, chargers []domain.Charger) bool {
	distToPickup := domain.HaversineDistance(currentLoc, shipment.PickupLocation)
	distPickupToDelivery := domain.HaversineDistance(shipment.PickupLocation, shipment.DeliveryLocation)
	distDeliveryToDepot := domain.HaversineDistance(shipment.DeliveryLocation, vehicle.DepotLocation)

	totalEnergy := (distToPickup + distPickupToDelivery + distDeliveryToDepot) * vehicle.EnergyConsumptionRate

	// Can do it directly.
	if currentCharge >= totalEnergy {
		return true
	}

	// Check if we can do it with a charging stop.
	if len(chargers) == 0 {
		return false
	}

	// Find nearest charger from current location.
	charger := s.findNearestCharger(currentLoc, chargers)
	distToCharger := domain.HaversineDistance(currentLoc, charger.Location)
	energyToCharger := distToCharger * vehicle.EnergyConsumptionRate

	// Can we reach the charger?
	if currentCharge < energyToCharger {
		return false
	}

	// After charging to full, can we complete the trip?
	distChargerToPickup := domain.HaversineDistance(charger.Location, shipment.PickupLocation)
	remainingEnergy := (distChargerToPickup + distPickupToDelivery + distDeliveryToDepot) * vehicle.EnergyConsumptionRate

	return vehicle.BatteryCapacityKWh >= remainingEnergy
}

// findNearestCharger returns the charger closest to the given location.
func (s *GreedySolver) findNearestCharger(loc domain.Location, chargers []domain.Charger) domain.Charger {
	best := chargers[0]
	bestDist := domain.HaversineDistance(loc, chargers[0].Location)

	for _, c := range chargers[1:] {
		d := domain.HaversineDistance(loc, c.Location)
		if d < bestDist {
			bestDist = d
			best = c
		}
	}

	return best
}
