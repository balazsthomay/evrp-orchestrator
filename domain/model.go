package domain

import "time"

// Location represents a geographic coordinate.
type Location struct {
	Latitude  float64
	Longitude float64
}

// Shipment represents a delivery request from a pickup to a delivery location.
type Shipment struct {
	ID               string
	PickupLocation   Location
	DeliveryLocation Location
	WeightKg         float64
	DeliveryStart    time.Time
	DeliveryEnd      time.Time
	ServiceTime      time.Duration
}

// Vehicle represents an electric vehicle with battery and payload constraints.
type Vehicle struct {
	ID                    string
	BatteryCapacityKWh    float64
	CurrentChargeKWh      float64
	EnergyConsumptionRate float64 // kWh per km
	MaxPayloadKg          float64
	DepotLocation         Location
	SpeedKmh              float64 // defaults to 60 if zero
}

// Charger represents a charging station with a fixed number of slots and power output.
type Charger struct {
	ID       string
	Location Location
	NumSlots int
	PowerKW  float64
}

// StopType enumerates the kinds of stops a vehicle can make along its route.
type StopType int

const (
	StopTypeDepot    StopType = iota
	StopTypePickup
	StopTypeDelivery
	StopTypeCharging
)

// Stop represents a single waypoint in a vehicle's route.
type Stop struct {
	Type                 StopType
	Location             Location
	ShipmentID           string
	ChargerID            string
	ArrivalTime          time.Time
	DepartureTime        time.Time
	ChargeOnArrivalKWh   float64
	ChargeOnDepartureKWh float64
	PayloadOnDepartureKg float64
}

// Route is the ordered sequence of stops assigned to a single vehicle.
type Route struct {
	VehicleID            string
	Stops                []Stop
	TotalDistanceKm      float64
	TotalDurationSeconds int64
	TotalEnergyKWh       float64
}

// Problem defines a complete eVRP instance to be solved.
type Problem struct {
	Shipments []Shipment
	Vehicles  []Vehicle
	Chargers  []Charger
	StartTime time.Time
	EndTime   time.Time
}

// Solution is the solver output: a set of routes plus aggregate metrics.
type Solution struct {
	Routes               []Route
	TotalDistanceKm      float64
	TotalDurationSeconds int64
	ShipmentsAssigned    int
	ShipmentsUnassigned  int
}
