package routing

import (
	"testing"

	"github.com/mahirpatel04/transit-route-optimizer/internal/gtfs"
)

func TestNearestStops_ReturnsClosestFirst(t *testing.T) {
	stops := []gtfs.Stop{
		{StopId: "far", Lat: 41.0, Lon: -74.5},
		{StopId: "near", Lat: 40.7581, Lon: -73.9856},
		{StopId: "medium", Lat: 40.80, Lon: -74.0},
	}

	got := nearestStops(stops, 40.7580, -73.9855, 2)

	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
	if got[0].StopId != "near" {
		t.Errorf("expected nearest stop first, got %s", got[0].StopId)
	}
	if got[1].StopId != "medium" {
		t.Errorf("expected second-nearest stop second, got %s", got[1].StopId)
	}
}

func TestNearestStops_CapsAtAvailableStops(t *testing.T) {
	stops := []gtfs.Stop{
		{StopId: "only", Lat: 40.7581, Lon: -73.9856},
	}

	got := nearestStops(stops, 40.7580, -73.9855, 3)

	if len(got) != 1 {
		t.Fatalf("expected 1 result (fewer than requested n), got %d", len(got))
	}
}
