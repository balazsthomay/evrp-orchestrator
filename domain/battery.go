package domain

import "time"

// EnergyRequired returns the energy in kWh needed to travel between two locations
// given the vehicle's energy consumption rate in kWh per km.
func EnergyRequired(a, b Location, consumptionRate float64) float64 {
	return HaversineDistance(a, b) * consumptionRate
}

// ChargingTime returns how long it takes to charge the given energy amount at
// the given power. Returns zero if either argument is non-positive.
func ChargingTime(energyKWh, powerKW float64) time.Duration {
	if energyKWh <= 0 || powerKW <= 0 {
		return 0
	}
	hours := energyKWh / powerKW
	return time.Duration(hours * float64(time.Hour))
}

// ChargeAfterTravel returns the remaining charge in kWh after traveling between
// two locations at the given consumption rate. The result may be negative if the
// vehicle does not have enough charge.
func ChargeAfterTravel(currentCharge float64, a, b Location, consumptionRate float64) float64 {
	return currentCharge - EnergyRequired(a, b, consumptionRate)
}

// CanReach returns whether a vehicle with the given charge can travel from a to b
// without the battery going below zero.
func CanReach(currentCharge float64, a, b Location, consumptionRate float64) bool {
	return ChargeAfterTravel(currentCharge, a, b, consumptionRate) >= 0
}
