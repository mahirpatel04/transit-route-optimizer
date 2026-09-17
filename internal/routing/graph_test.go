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
	at := testWeekday(t, conn)

	edges, err := expand(ctx, conn, "127N", at, map[time.Time][]string{})
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

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}

	weekday := testWeekday(t, conn)
	nextDay := weekday.AddDate(0, 0, 1)

	// 1:15 AM local time on the day after our test weekday: trips still
	// running from the previous day's service (GTFS time >= 24:00:00) must
	// be considered, not just that day's own early-morning trips.
	at := time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), 1, 15, 0, 0, loc)

	edges, err := expand(ctx, conn, "127N", at, map[time.Time][]string{})
	if err != nil {
		t.Fatalf("expand: %v", err)
	}

	// At least one ride edge should come from the PREVIOUS calendar day's
	// service still running past midnight (GTFS time >= 24:00:00) — if
	// rollover handling were broken or deleted, only the literal day's own
	// (sparse, early-morning) service would be considered. A rolled-over
	// trip's real-world DepartAt naturally falls on the *next* calendar day
	// in local wall-clock terms (that's what "past midnight" means), so we
	// check it against the previous day's midnight using a >= 24h GTFS
	// offset rather than comparing calendar days directly.
	sawPreviousDayEdge := false
	previousDayMidnight := time.Date(weekday.Year(), weekday.Month(), weekday.Day(), 0, 0, 0, 0, loc)
	for _, e := range edges {
		if e.Kind == edgeKindRide && e.DepartAt.Sub(previousDayMidnight) >= 24*time.Hour {
			sawPreviousDayEdge = true
			break
		}
	}
	if !sawPreviousDayEdge {
		t.Error("expected at least one ride edge using the previous day's rolled-over service (GTFS time >= 24:00:00)")
	}
}

func TestServiceWindowsFor_ConvertsToAgencyTimezone(t *testing.T) {
	// 2026-09-17 02:00 UTC is 2026-09-16 22:00 America/New_York (EDT, UTC-4).
	// A buggy implementation that treats this as UTC would compute the
	// wrong calendar date and time-of-day entirely.
	at := time.Date(2026, 9, 17, 2, 0, 0, 0, time.UTC)

	windows := serviceWindowsFor(at)

	loc, _ := time.LoadLocation("America/New_York")
	wantDate := time.Date(2026, 9, 16, 0, 0, 0, 0, loc)
	if !windows[0].date.Equal(wantDate) {
		t.Errorf("expected window 0 date %v, got %v", wantDate, windows[0].date)
	}
	wantTimeOfDay := 22 * time.Hour
	if windows[0].gtfsTimeFrom != wantTimeOfDay {
		t.Errorf("expected window 0 time-of-day %v, got %v", wantTimeOfDay, windows[0].gtfsTimeFrom)
	}
}
