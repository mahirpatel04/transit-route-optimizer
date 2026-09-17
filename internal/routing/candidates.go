package routing

import (
	"sort"

	"github.com/mahirpatel04/transit-route-optimizer/internal/geo"
	"github.com/mahirpatel04/transit-route-optimizer/internal/gtfs"
)

// nearestStops returns the n closest stops to (lat, lon), sorted
// nearest-first. Returns fewer than n if stops has fewer entries.
func nearestStops(stops []gtfs.Stop, lat, lon float64, n int) []gtfs.Stop {
	sorted := make([]gtfs.Stop, len(stops))
	copy(sorted, stops)

	sort.Slice(sorted, func(i, j int) bool {
		di := geo.Haversine(lat, lon, sorted[i].Lat, sorted[i].Lon)
		dj := geo.Haversine(lat, lon, sorted[j].Lat, sorted[j].Lon)
		return di < dj
	})

	if len(sorted) > n {
		sorted = sorted[:n]
	}
	return sorted
}
