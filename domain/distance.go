package domain

import (
	"math"
	"time"
)

const earthRadiusKm = 6371.0

// HaversineDistance returns the great-circle distance in kilometers between two
// geographic coordinates using the Haversine formula.
func HaversineDistance(a, b Location) float64 {
	lat1 := degreesToRadians(a.Latitude)
	lat2 := degreesToRadians(b.Latitude)
	dLat := degreesToRadians(b.Latitude - a.Latitude)
	dLon := degreesToRadians(b.Longitude - a.Longitude)

	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)

	c := 2 * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))

	return earthRadiusKm * c
}

// TravelTime returns the duration needed to travel from a to b at the given
// speed in km/h. If speedKmh is zero or negative, a default of 60 km/h is used.
func TravelTime(a, b Location, speedKmh float64) time.Duration {
	if speedKmh <= 0 {
		speedKmh = 60.0
	}
	dist := HaversineDistance(a, b)
	hours := dist / speedKmh
	return time.Duration(hours * float64(time.Hour))
}

func degreesToRadians(deg float64) float64 {
	return deg * math.Pi / 180.0
}
