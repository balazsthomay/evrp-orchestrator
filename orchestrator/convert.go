package orchestrator

import (
	"time"

	evrpv1 "github.com/thomaybalazs/evrp-orchestrator/gen/einride/evrp/v1"
	"github.com/thomaybalazs/evrp-orchestrator/domain"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// protoProblemToDomain converts a proto Problem to a domain Problem.
func protoProblemToDomain(p *evrpv1.Problem) *domain.Problem {
	shipments := make([]domain.Shipment, len(p.GetShipments()))
	for i, s := range p.GetShipments() {
		shipments[i] = domain.Shipment{
			ID:               s.GetShipmentId(),
			PickupLocation:   protoLocationToDomain(s.GetPickupLocation()),
			DeliveryLocation: protoLocationToDomain(s.GetDeliveryLocation()),
			WeightKg:         s.GetWeightKg(),
			DeliveryStart:    s.GetDeliveryWindow().GetStartTime().AsTime(),
			DeliveryEnd:      s.GetDeliveryWindow().GetEndTime().AsTime(),
			ServiceTime:      time.Duration(s.GetServiceTimeSeconds()) * time.Second,
		}
	}

	vehicles := make([]domain.Vehicle, len(p.GetVehicles()))
	for i, v := range p.GetVehicles() {
		vehicles[i] = domain.Vehicle{
			ID:                    v.GetVehicleId(),
			BatteryCapacityKWh:    v.GetBatteryCapacityKwh(),
			CurrentChargeKWh:      v.GetCurrentChargeKwh(),
			EnergyConsumptionRate: v.GetEnergyConsumptionRate(),
			MaxPayloadKg:          v.GetMaxPayloadKg(),
			DepotLocation:         protoLocationToDomain(v.GetDepotLocation()),
			SpeedKmh:              v.GetSpeedKmh(),
		}
	}

	chargers := make([]domain.Charger, len(p.GetChargers()))
	for i, c := range p.GetChargers() {
		chargers[i] = domain.Charger{
			ID:       c.GetChargerId(),
			Location: protoLocationToDomain(c.GetLocation()),
			NumSlots: int(c.GetNumSlots()),
			PowerKW:  c.GetPowerKw(),
		}
	}

	return &domain.Problem{
		Shipments: shipments,
		Vehicles:  vehicles,
		Chargers:  chargers,
		StartTime: p.GetStartTime().AsTime(),
		EndTime:   p.GetEndTime().AsTime(),
	}
}

func protoLocationToDomain(l *evrpv1.Location) domain.Location {
	if l == nil {
		return domain.Location{}
	}
	return domain.Location{
		Latitude:  l.GetLatitude(),
		Longitude: l.GetLongitude(),
	}
}

// domainSolutionToProto converts a domain Solution to a proto Solution.
func domainSolutionToProto(s *domain.Solution, problemName string) *evrpv1.Solution {
	routes := make([]*evrpv1.Route, len(s.Routes))
	for i, r := range s.Routes {
		stops := make([]*evrpv1.Stop, len(r.Stops))
		for j, st := range r.Stops {
			stops[j] = &evrpv1.Stop{
				StopType:             domainStopTypeToProto(st.Type),
				Location:             domainLocationToProto(st.Location),
				ShipmentId:           st.ShipmentID,
				ChargerId:            st.ChargerID,
				ArrivalTime:          timestamppb.New(st.ArrivalTime),
				DepartureTime:        timestamppb.New(st.DepartureTime),
				ChargeOnArrivalKwh:   st.ChargeOnArrivalKWh,
				ChargeOnDepartureKwh: st.ChargeOnDepartureKWh,
				PayloadOnDepartureKg: st.PayloadOnDepartureKg,
			}
		}
		routes[i] = &evrpv1.Route{
			VehicleId:            r.VehicleID,
			Stops:                stops,
			TotalDistanceKm:      r.TotalDistanceKm,
			TotalDurationSeconds: r.TotalDurationSeconds,
			TotalEnergyKwh:       r.TotalEnergyKWh,
		}
	}

	return &evrpv1.Solution{
		Name:                 problemName + "/solution",
		Routes:               routes,
		TotalDistanceKm:      s.TotalDistanceKm,
		TotalDurationSeconds: s.TotalDurationSeconds,
		ShipmentsAssigned:    int32(s.ShipmentsAssigned),
		ShipmentsUnassigned:  int32(s.ShipmentsUnassigned),
		CreateTime:           timestamppb.Now(),
	}
}

func domainLocationToProto(l domain.Location) *evrpv1.Location {
	return &evrpv1.Location{
		Latitude:  l.Latitude,
		Longitude: l.Longitude,
	}
}

func domainStopTypeToProto(st domain.StopType) evrpv1.Stop_StopType {
	switch st {
	case domain.StopTypeDepot:
		return evrpv1.Stop_STOP_TYPE_DEPOT
	case domain.StopTypePickup:
		return evrpv1.Stop_STOP_TYPE_PICKUP
	case domain.StopTypeDelivery:
		return evrpv1.Stop_STOP_TYPE_DELIVERY
	case domain.StopTypeCharging:
		return evrpv1.Stop_STOP_TYPE_CHARGING
	default:
		return evrpv1.Stop_STOP_TYPE_UNSPECIFIED
	}
}
