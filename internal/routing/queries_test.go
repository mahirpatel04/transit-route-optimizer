package routing

import (
	"context"
	"testing"
	"time"
)

func TestNextDeparturesByRoute_FindsAtLeastOneRoute(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()

	serviceIDs, err := activeServiceIDs(ctx, conn, testWeekday(t, conn))
	if err != nil {
		t.Fatalf("activeServiceIDs: %v", err)
	}
	if len(serviceIDs) == 0 {
		t.Skip("no active services for the test date in this GTFS snapshot")
	}

	// Times Sq-42 St, northbound platform on the 1/2/3 line.
	candidates, err := nextDeparturesByRoute(ctx, conn, "127N", 8*time.Hour, serviceIDs)
	if err != nil {
		t.Fatalf("nextDeparturesByRoute: %v", err)
	}
	if len(candidates) == 0 {
		t.Fatal("expected at least one candidate trip departing after 8am from a major stop")
	}
	for _, c := range candidates {
		if c.DepartureTime < 8*time.Hour {
			t.Errorf("candidate departs at %v, before the requested 8h floor", c.DepartureTime)
		}
	}
}

func TestNextDeparturesByRoute_NoServiceReturnsEmpty(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()

	candidates, err := nextDeparturesByRoute(ctx, conn, "127N", 8*time.Hour, []string{"no-such-service-id"})
	if err != nil {
		t.Fatalf("nextDeparturesByRoute: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected no candidates for a nonexistent service_id, got %v", candidates)
	}
}

func TestNextStopOnTrip_FindsTheNextStop(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()

	serviceIDs, err := activeServiceIDs(ctx, conn, testWeekday(t, conn))
	if err != nil {
		t.Fatalf("activeServiceIDs: %v", err)
	}
	if len(serviceIDs) == 0 {
		t.Skip("no active services for the test date in this GTFS snapshot")
	}

	candidates, err := nextDeparturesByRoute(ctx, conn, "127N", 8*time.Hour, serviceIDs)
	if err != nil || len(candidates) == 0 {
		t.Skip("no candidate trips available to test against")
	}

	next, ok, err := nextStopOnTrip(ctx, conn, candidates[0].TripID, candidates[0].StopSequence)
	if err != nil {
		t.Fatalf("nextStopOnTrip: %v", err)
	}
	if !ok {
		t.Fatal("expected a next stop to exist for a trip that just departed 127N (unlikely to be the last stop)")
	}
	if next.StopID == "" {
		t.Error("expected a non-empty next stop id")
	}
}

func TestNextStopOnTrip_EndOfTripReturnsFalse(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()

	// Directly probe a made-up trip id past any real sequence
	// (stop_sequence 999999 can't exist on any real trip).
	_, ok, err := nextStopOnTrip(ctx, conn, "no-such-trip-id", 999999)
	if err != nil {
		t.Fatalf("nextStopOnTrip: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false for a nonexistent trip/sequence")
	}
}
