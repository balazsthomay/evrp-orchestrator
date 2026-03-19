package domain

import (
	"math"
	"testing"
	"time"
)

func TestEnergyRequired(t *testing.T) {
	// Two points approximately 100 km apart.
	a := Location{Latitude: 0, Longitude: 0}
	b := Location{Latitude: 0.9009, Longitude: 0}
	dist := HaversineDistance(a, b)

	tests := []struct {
		name            string
		a               Location
		b               Location
		consumptionRate float64
		wantKWh         float64
		tolerance       float64
	}{
		{
			name:            "100 km at 0.2 kWh/km equals 20 kWh",
			a:               a,
			b:               b,
			consumptionRate: 0.2,
			wantKWh:         dist * 0.2,
			tolerance:       0.5,
		},
		{
			name:            "same point needs zero energy",
			a:               a,
			b:               a,
			consumptionRate: 0.2,
			wantKWh:         0,
			tolerance:       0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EnergyRequired(tt.a, tt.b, tt.consumptionRate)
			if math.Abs(got-tt.wantKWh) > tt.tolerance {
				t.Errorf("EnergyRequired() = %.4f kWh, want %.4f kWh (tolerance ±%.4f)",
					got, tt.wantKWh, tt.tolerance)
			}
		})
	}
}

func TestChargingTime(t *testing.T) {
	tests := []struct {
		name      string
		energyKWh float64
		powerKW   float64
		want      time.Duration
		tolerance time.Duration
	}{
		{
			name:      "20 kWh at 50 kW takes 24 minutes",
			energyKWh: 20,
			powerKW:   50,
			want:      24 * time.Minute,
			tolerance: time.Second,
		},
		{
			name:      "zero energy needs no time",
			energyKWh: 0,
			powerKW:   50,
			want:      0,
			tolerance: 0,
		},
		{
			name:      "negative energy returns zero",
			energyKWh: -5,
			powerKW:   50,
			want:      0,
			tolerance: 0,
		},
		{
			name:      "zero power returns zero",
			energyKWh: 20,
			powerKW:   0,
			want:      0,
			tolerance: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ChargingTime(tt.energyKWh, tt.powerKW)
			diff := got - tt.want
			if diff < 0 {
				diff = -diff
			}
			if diff > tt.tolerance {
				t.Errorf("ChargingTime(%.1f, %.1f) = %v, want %v (tolerance ±%v)",
					tt.energyKWh, tt.powerKW, got, tt.want, tt.tolerance)
			}
		})
	}
}

func TestCanReach(t *testing.T) {
	// Two points approximately 100 km apart.
	a := Location{Latitude: 0, Longitude: 0}
	b := Location{Latitude: 0.9009, Longitude: 0}
	// At 0.2 kWh/km, 100 km needs ~20 kWh.

	tests := []struct {
		name            string
		currentCharge   float64
		a               Location
		b               Location
		consumptionRate float64
		want            bool
	}{
		{
			name:            "sufficient charge",
			currentCharge:   25,
			a:               a,
			b:               b,
			consumptionRate: 0.2,
			want:            true,
		},
		{
			name:            "barely enough charge",
			currentCharge:   21, // ~100 km at 0.2 kWh/km needs ~20 kWh; 21 is enough
			a:               a,
			b:               b,
			consumptionRate: 0.2,
			want:            true,
		},
		{
			name:            "insufficient charge",
			currentCharge:   15,
			a:               a,
			b:               b,
			consumptionRate: 0.2,
			want:            false,
		},
		{
			name:            "same location always reachable",
			currentCharge:   0,
			a:               a,
			b:               a,
			consumptionRate: 0.2,
			want:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanReach(tt.currentCharge, tt.a, tt.b, tt.consumptionRate)
			if got != tt.want {
				t.Errorf("CanReach(%.1f, %v, %v, %.1f) = %v, want %v",
					tt.currentCharge, tt.a, tt.b, tt.consumptionRate, got, tt.want)
			}
		})
	}
}

func TestChargeAfterTravel(t *testing.T) {
	a := Location{Latitude: 0, Longitude: 0}
	b := Location{Latitude: 0.9009, Longitude: 0}
	dist := HaversineDistance(a, b)

	tests := []struct {
		name            string
		currentCharge   float64
		a               Location
		b               Location
		consumptionRate float64
		wantCharge      float64
		tolerance       float64
	}{
		{
			name:            "charge remaining after 100 km trip",
			currentCharge:   50,
			a:               a,
			b:               b,
			consumptionRate: 0.2,
			wantCharge:      50 - dist*0.2,
			tolerance:       0.5,
		},
		{
			name:            "negative charge when insufficient",
			currentCharge:   10,
			a:               a,
			b:               b,
			consumptionRate: 0.2,
			wantCharge:      10 - dist*0.2,
			tolerance:       0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ChargeAfterTravel(tt.currentCharge, tt.a, tt.b, tt.consumptionRate)
			if math.Abs(got-tt.wantCharge) > tt.tolerance {
				t.Errorf("ChargeAfterTravel() = %.4f, want %.4f (tolerance ±%.4f)",
					got, tt.wantCharge, tt.tolerance)
			}
		})
	}
}
