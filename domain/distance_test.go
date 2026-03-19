package domain

import (
	"math"
	"testing"
	"time"
)

func TestHaversineDistance(t *testing.T) {
	tests := []struct {
		name      string
		a         Location
		b         Location
		wantKm    float64
		tolerance float64
	}{
		{
			name:      "Stockholm to Gothenburg",
			a:         Location{Latitude: 59.3293, Longitude: 18.0686},
			b:         Location{Latitude: 57.7089, Longitude: 11.9746},
			wantKm:    397,
			tolerance: 5,
		},
		{
			name:      "same point returns zero",
			a:         Location{Latitude: 40.7128, Longitude: -74.0060},
			b:         Location{Latitude: 40.7128, Longitude: -74.0060},
			wantKm:    0,
			tolerance: 0.001,
		},
		{
			name:      "antipodal points",
			a:         Location{Latitude: 0, Longitude: 0},
			b:         Location{Latitude: 0, Longitude: 180},
			wantKm:    20015,
			tolerance: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HaversineDistance(tt.a, tt.b)
			if math.Abs(got-tt.wantKm) > tt.tolerance {
				t.Errorf("HaversineDistance(%v, %v) = %.2f km, want %.2f km (tolerance ±%.2f)",
					tt.a, tt.b, got, tt.wantKm, tt.tolerance)
			}
		})
	}
}

func TestTravelTime(t *testing.T) {
	tests := []struct {
		name     string
		a        Location
		b        Location
		speedKmh float64
		wantMin  float64 // expected travel time in minutes
		tolerance float64 // tolerance in minutes
	}{
		{
			name:      "100 km at 60 km/h is 100 minutes",
			a:         Location{Latitude: 0, Longitude: 0},
			b:         Location{Latitude: 0.9009, Longitude: 0}, // ~100 km along equator
			speedKmh:  60,
			wantMin:   100,
			tolerance: 2,
		},
		{
			name:      "zero speed defaults to 60 km/h",
			a:         Location{Latitude: 0, Longitude: 0},
			b:         Location{Latitude: 0.9009, Longitude: 0},
			speedKmh:  0,
			wantMin:   100,
			tolerance: 2,
		},
		{
			name:      "same point returns zero",
			a:         Location{Latitude: 10, Longitude: 20},
			b:         Location{Latitude: 10, Longitude: 20},
			speedKmh:  100,
			wantMin:   0,
			tolerance: 0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TravelTime(tt.a, tt.b, tt.speedKmh)
			gotMin := got.Minutes()
			if math.Abs(gotMin-tt.wantMin) > tt.tolerance {
				t.Errorf("TravelTime(%v, %v, %.1f) = %v (%.2f min), want ~%.2f min (tolerance ±%.2f)",
					tt.a, tt.b, tt.speedKmh, got.Round(time.Second), gotMin, tt.wantMin, tt.tolerance)
			}
		})
	}
}
