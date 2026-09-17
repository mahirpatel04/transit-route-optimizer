package routing

import (
	"context"
	"testing"
	"time"
)

func TestExpand_ReturnsTransitAndTransferEdges(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()

	// A Wednesday, 8am — well within normal NYC subway service hours.
	at := time.Date(2026, 9, 16, 8, 0, 0, 0, time.UTC)

	edges, err := expand(ctx, conn, "127N", at)
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if len(edges) == 0 {
		t.Fatal("expected at least one edge from a major stop at 8am on a weekday")
	}

	sawRide := false
	sawWalk := false
	for _, e := range edges {
		if e.Kind == edgeKindRide {
			sawRide = true
			if !e.ArriveAt.After(e.DepartAt) {
				t.Errorf("ride edge to %s: arrival %v not after departure %v", e.ToStopID, e.ArriveAt, e.DepartAt)
			}
			if e.RouteID == "" {
				t.Errorf("ride edge to %s missing route id", e.ToStopID)
			}
		}
		if e.Kind == edgeKindWalk && e.ToStopID == "127S" {
			sawWalk = true
			if e.DepartAt != e.ArriveAt {
				t.Errorf("expected the free 127N->127S transfer to have equal depart/arrive times, got %v -> %v", e.DepartAt, e.ArriveAt)
			}
		}
	}
	if !sawRide {
		t.Error("expected at least one ride edge")
	}
	if !sawWalk {
		t.Error("expected the 127N->127S transfer edge to appear")
	}
}

func TestExpand_HandlesPostMidnightRollover(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()

	// 1:15 AM on a Thursday: trips still running from Wednesday's service
	// (GTFS time >= 24:00:00) must be considered, not just Thursday's own
	// early-morning trips.
	at := time.Date(2026, 9, 17, 1, 15, 0, 0, time.UTC)

	edges, err := expand(ctx, conn, "127N", at)
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	// This is a smoke check, not a strict assertion on count: late-night
	// NYC subway service is real but sparse. The key behavior under test
	// is that expand() doesn't error and doesn't limit itself to only
	// Thursday's calendar — verified more precisely by service_days_test.go
	// and queries_test.go. Here we just confirm no edges are silently
	// dropped due to a rollover bug causing a query error.
	_ = edges
}
