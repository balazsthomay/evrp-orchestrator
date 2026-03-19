package domain

import (
	"errors"
	"fmt"
)

// CheckRouteFeasibility validates that a route is feasible given the vehicle
// constraints. It returns nil if the route is feasible, or an error describing
// the first violation found.
//
// Checks performed:
//  1. Route starts and ends at the vehicle's depot.
//  2. Battery charge never goes below zero at any stop.
//  3. Payload never exceeds the vehicle's maximum at any stop.
func CheckRouteFeasibility(route Route, vehicle Vehicle) error {
	if len(route.Stops) == 0 {
		return errors.New("route has no stops")
	}

	// Check that the route starts at the depot.
	first := route.Stops[0]
	if first.Type != StopTypeDepot || !locationsEqual(first.Location, vehicle.DepotLocation) {
		return fmt.Errorf("route must start at depot %+v, but starts at %+v",
			vehicle.DepotLocation, first.Location)
	}

	// Check that the route ends at the depot.
	last := route.Stops[len(route.Stops)-1]
	if last.Type != StopTypeDepot || !locationsEqual(last.Location, vehicle.DepotLocation) {
		return fmt.Errorf("route must end at depot %+v, but ends at %+v",
			vehicle.DepotLocation, last.Location)
	}

	// Walk the stops and verify battery and payload constraints.
	for i, stop := range route.Stops {
		if stop.ChargeOnArrivalKWh < 0 {
			return fmt.Errorf("battery is negative (%.2f kWh) on arrival at stop %d",
				stop.ChargeOnArrivalKWh, i)
		}
		if stop.ChargeOnDepartureKWh < 0 {
			return fmt.Errorf("battery is negative (%.2f kWh) on departure from stop %d",
				stop.ChargeOnDepartureKWh, i)
		}
		if stop.PayloadOnDepartureKg > vehicle.MaxPayloadKg {
			return fmt.Errorf("payload (%.2f kg) exceeds vehicle max (%.2f kg) at stop %d",
				stop.PayloadOnDepartureKg, vehicle.MaxPayloadKg, i)
		}
	}

	return nil
}

// locationsEqual returns true if two locations are identical.
func locationsEqual(a, b Location) bool {
	return a.Latitude == b.Latitude && a.Longitude == b.Longitude
}
