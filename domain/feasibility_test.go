package domain

import (
	"testing"
)

func TestCheckRouteFeasibility(t *testing.T) {
	depot := Location{Latitude: 59.3293, Longitude: 18.0686}

	vehicle := Vehicle{
		ID:                    "v1",
		BatteryCapacityKWh:    80,
		CurrentChargeKWh:      80,
		EnergyConsumptionRate: 0.2,
		MaxPayloadKg:          1000,
		DepotLocation:         depot,
		SpeedKmh:              60,
	}

	tests := []struct {
		name    string
		route   Route
		vehicle Vehicle
		wantErr bool
	}{
		{
			name: "valid route with depot start and end",
			route: Route{
				VehicleID: "v1",
				Stops: []Stop{
					{
						Type:                 StopTypeDepot,
						Location:             depot,
						ChargeOnArrivalKWh:   80,
						ChargeOnDepartureKWh: 80,
						PayloadOnDepartureKg: 50,
					},
					{
						Type:                 StopTypePickup,
						Location:             Location{Latitude: 59.4, Longitude: 18.1},
						ShipmentID:           "s1",
						ChargeOnArrivalKWh:   75,
						ChargeOnDepartureKWh: 75,
						PayloadOnDepartureKg: 50,
					},
					{
						Type:                 StopTypeDelivery,
						Location:             Location{Latitude: 59.5, Longitude: 18.2},
						ShipmentID:           "s1",
						ChargeOnArrivalKWh:   70,
						ChargeOnDepartureKWh: 70,
						PayloadOnDepartureKg: 0,
					},
					{
						Type:                 StopTypeDepot,
						Location:             depot,
						ChargeOnArrivalKWh:   65,
						ChargeOnDepartureKWh: 65,
						PayloadOnDepartureKg: 0,
					},
				},
			},
			vehicle: vehicle,
			wantErr: false,
		},
		{
			name: "battery goes negative on arrival",
			route: Route{
				VehicleID: "v1",
				Stops: []Stop{
					{
						Type:                 StopTypeDepot,
						Location:             depot,
						ChargeOnArrivalKWh:   80,
						ChargeOnDepartureKWh: 80,
						PayloadOnDepartureKg: 0,
					},
					{
						Type:                 StopTypePickup,
						Location:             Location{Latitude: 60.0, Longitude: 19.0},
						ShipmentID:           "s1",
						ChargeOnArrivalKWh:   -5,
						ChargeOnDepartureKWh: -5,
						PayloadOnDepartureKg: 100,
					},
					{
						Type:                 StopTypeDepot,
						Location:             depot,
						ChargeOnArrivalKWh:   -15,
						ChargeOnDepartureKWh: -15,
						PayloadOnDepartureKg: 0,
					},
				},
			},
			vehicle: vehicle,
			wantErr: true,
		},
		{
			name: "payload exceeds vehicle max",
			route: Route{
				VehicleID: "v1",
				Stops: []Stop{
					{
						Type:                 StopTypeDepot,
						Location:             depot,
						ChargeOnArrivalKWh:   80,
						ChargeOnDepartureKWh: 80,
						PayloadOnDepartureKg: 0,
					},
					{
						Type:                 StopTypePickup,
						Location:             Location{Latitude: 59.4, Longitude: 18.1},
						ShipmentID:           "s1",
						ChargeOnArrivalKWh:   75,
						ChargeOnDepartureKWh: 75,
						PayloadOnDepartureKg: 1500, // exceeds 1000 kg max
					},
					{
						Type:                 StopTypeDepot,
						Location:             depot,
						ChargeOnArrivalKWh:   70,
						ChargeOnDepartureKWh: 70,
						PayloadOnDepartureKg: 0,
					},
				},
			},
			vehicle: vehicle,
			wantErr: true,
		},
		{
			name: "route does not start at depot",
			route: Route{
				VehicleID: "v1",
				Stops: []Stop{
					{
						Type:                 StopTypePickup,
						Location:             Location{Latitude: 59.4, Longitude: 18.1},
						ShipmentID:           "s1",
						ChargeOnArrivalKWh:   80,
						ChargeOnDepartureKWh: 80,
						PayloadOnDepartureKg: 50,
					},
					{
						Type:                 StopTypeDepot,
						Location:             depot,
						ChargeOnArrivalKWh:   75,
						ChargeOnDepartureKWh: 75,
						PayloadOnDepartureKg: 0,
					},
				},
			},
			vehicle: vehicle,
			wantErr: true,
		},
		{
			name: "route does not end at depot",
			route: Route{
				VehicleID: "v1",
				Stops: []Stop{
					{
						Type:                 StopTypeDepot,
						Location:             depot,
						ChargeOnArrivalKWh:   80,
						ChargeOnDepartureKWh: 80,
						PayloadOnDepartureKg: 50,
					},
					{
						Type:                 StopTypeDelivery,
						Location:             Location{Latitude: 59.5, Longitude: 18.2},
						ShipmentID:           "s1",
						ChargeOnArrivalKWh:   70,
						ChargeOnDepartureKWh: 70,
						PayloadOnDepartureKg: 0,
					},
				},
			},
			vehicle: vehicle,
			wantErr: true,
		},
		{
			name:    "empty route",
			route:   Route{VehicleID: "v1"},
			vehicle: vehicle,
			wantErr: true,
		},
		{
			name: "battery negative on departure",
			route: Route{
				VehicleID: "v1",
				Stops: []Stop{
					{
						Type:                 StopTypeDepot,
						Location:             depot,
						ChargeOnArrivalKWh:   80,
						ChargeOnDepartureKWh: -1,
						PayloadOnDepartureKg: 0,
					},
					{
						Type:                 StopTypeDepot,
						Location:             depot,
						ChargeOnArrivalKWh:   60,
						ChargeOnDepartureKWh: 60,
						PayloadOnDepartureKg: 0,
					},
				},
			},
			vehicle: vehicle,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckRouteFeasibility(tt.route, tt.vehicle)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckRouteFeasibility() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
