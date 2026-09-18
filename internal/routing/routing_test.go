package routing

import (
	"context"
	"testing"
	"time"
)

func TestFindRoute_TimesSqToGrandCentral(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()

	// A Wednesday, 8am. 127N (Times Sq-42 St, 1/2/3) to a Grand Central-42 St
	// platform (631N or 631S, on the 4/5/6) requires at least one transfer.
	// Only platform-level stops ever appear in stop_times — bare parent
	// stations like "631" never do, so they can never be a reachable
	// destination in this graph; either direction platform is a valid
	// arrival at the same physical station.
	departAt := testWeekday(t, conn)

	route, err := FindRoute(ctx, conn, "127N", "631S", departAt)
	if err != nil {
		t.Fatalf("FindRoute: %v", err)
	}
	if len(route.Legs) == 0 {
		t.Fatal("expected at least one leg")
	}
	if route.TotalTime <= 0 {
		t.Errorf("expected positive total time, got %v", route.TotalTime)
	}
	if route.TotalTime > 2*time.Hour {
		t.Errorf("expected a same-borough NYC subway trip under 2 hours, got %v", route.TotalTime)
	}

	first := route.Legs[0]
	if first.FromStopID != "127N" {
		t.Errorf("expected the route to start at 127N, got %s", first.FromStopID)
	}
	last := route.Legs[len(route.Legs)-1]
	if last.ToStopID != "631S" {
		t.Errorf("expected the route to end at 631S, got %s", last.ToStopID)
	}

	for i, leg := range route.Legs {
		if !leg.ArriveAt.After(leg.DepartAt) && leg.Kind == LegKindRide {
			t.Errorf("leg %d: ride arrival %v not after departure %v", i, leg.ArriveAt, leg.DepartAt)
		}
		if i > 0 && route.Legs[i-1].ToStopID != leg.FromStopID {
			t.Errorf("leg %d: discontinuous path, previous leg ended at %s but this one starts at %s",
				i, route.Legs[i-1].ToStopID, leg.FromStopID)
		}
	}
}

func TestFindRoute_UnionSquareToTimesSquare(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()

	departAt := testWeekday(t, conn)

	route, err := FindRoute(ctx, conn, "635N", "127N", departAt)
	if err != nil {
		t.Fatalf("FindRoute: %v", err)
	}
	if len(route.Legs) == 0 {
		t.Fatal("expected at least one leg")
	}
	if route.TotalTime <= 0 || route.TotalTime > 20*time.Minute {
		t.Errorf("expected a same-borough trip (Union Sq to Times Sq), got %v", route.TotalTime)
	}
}

func TestFindRouteMultiTarget_ReachesFastestOfSeveralTargets(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()

	departAt := testWeekday(t, conn)

	// 631N and 631S are both Grand Central-42 St platforms (4/5/6) — either
	// is a real, valid arrival at the same physical station.
	route, err := FindRouteMultiTarget(ctx, conn, "127N", []string{"631N", "631S"}, departAt)
	if err != nil {
		t.Fatalf("FindRouteMultiTarget: %v", err)
	}
	if len(route.Legs) == 0 {
		t.Fatal("expected at least one leg")
	}
	last := route.Legs[len(route.Legs)-1]
	if last.ToStopID != "631N" && last.ToStopID != "631S" {
		t.Errorf("expected the route to end at one of the requested targets, got %s", last.ToStopID)
	}
	if route.TotalTime <= 0 || route.TotalTime > 2*time.Hour {
		t.Errorf("expected a positive, same-borough trip time, got %v", route.TotalTime)
	}
}

func TestFindRouteMultiTarget_OriginAlreadyATargetReturnsEmptyRoute(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()

	departAt := testWeekday(t, conn)

	route, err := FindRouteMultiTarget(ctx, conn, "127N", []string{"631S", "127N"}, departAt)
	if err != nil {
		t.Fatalf("FindRouteMultiTarget: %v", err)
	}
	if len(route.Legs) != 0 {
		t.Errorf("expected no legs when the origin is already one of the targets, got %d", len(route.Legs))
	}
}

func TestFindRoute_SameStopReturnsEmptyRoute(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()

	departAt := testWeekday(t, conn)

	route, err := FindRoute(ctx, conn, "127N", "127N", departAt)
	if err != nil {
		t.Fatalf("FindRoute: %v", err)
	}
	if len(route.Legs) != 0 {
		t.Errorf("expected no legs when origin equals destination, got %d", len(route.Legs))
	}
	if route.TotalTime != 0 {
		t.Errorf("expected 0 total time when origin equals destination, got %v", route.TotalTime)
	}
}
