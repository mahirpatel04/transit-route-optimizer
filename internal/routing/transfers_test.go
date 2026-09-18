package routing

import (
	"context"
	"testing"
	"time"
)

func TestTransferNeighbors_SameParentStationIsFree(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()

	// 127N and 127S share parent_station "127" (Times Sq-42 St, one line's
	// two direction platforms).
	transfers, err := transferNeighbors(ctx, conn, "127N")
	if err != nil {
		t.Fatalf("transferNeighbors: %v", err)
	}

	found := false
	for _, tr := range transfers {
		if tr.StopID == "127S" {
			found = true
			if tr.Cost != 0 {
				t.Errorf("expected 0 cost between same-parent_station platforms, got %v", tr.Cost)
			}
		}
	}
	if !found {
		t.Fatal("expected 127S to appear as a same-parent_station transfer from 127N")
	}
}

func TestTransferNeighbors_NearbyParentStationCostsProportionalToDistance(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()

	// 127 and 902 are different lines' parent stations, both named
	// "Times Sq-42 St" — within crossComplexTransferRadiusMeters regardless
	// of the name match, so this also covers same-named nearby complexes.
	transfers, err := transferNeighbors(ctx, conn, "127")
	if err != nil {
		t.Fatalf("transferNeighbors: %v", err)
	}

	found := false
	for _, tr := range transfers {
		if tr.StopID == "902" {
			found = true
			var radiusMeters float64 = crossComplexTransferRadiusMeters
			maxCost := time.Duration(radiusMeters/pedestrianSpeedMetersPerSecond) * time.Second
			if tr.Cost <= 0 || tr.Cost > maxCost {
				t.Errorf("expected a positive cost bounded by the transfer radius at walking pace, got %v", tr.Cost)
			}
		}
	}
	if !found {
		t.Fatal("expected 902 to appear as a nearby cross-complex transfer from 127")
	}
}

func TestTransferNeighbors_NearbyDifferentlyNamedComplexIsConnected(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()

	// 719 (Court Sq, 7 train) and F09 (Court Sq-23 St, E/M train) are the
	// same real-world transfer complex but have different stop_names —
	// matching by name alone would miss this real transfer.
	transfers, err := transferNeighbors(ctx, conn, "719")
	if err != nil {
		t.Fatalf("transferNeighbors: %v", err)
	}

	found := false
	for _, tr := range transfers {
		if tr.StopID == "F09" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected F09 (Court Sq-23 St) to appear as a nearby cross-complex transfer from 719 (Court Sq), despite the differing stop_name")
	}
}

func TestTransferNeighbors_DistantComplexIsNotConnected(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()

	// 127 (Times Sq-42 St) and R20 (Union Square) are both parent stations
	// but nowhere near each other — must not be connected regardless of
	// dropping the stop_name requirement.
	transfers, err := transferNeighbors(ctx, conn, "127")
	if err != nil {
		t.Fatalf("transferNeighbors: %v", err)
	}

	for _, tr := range transfers {
		if tr.StopID == "R20" {
			t.Fatalf("expected R20 (Union Square) not to appear as a transfer from 127 (Times Sq-42 St) — too far apart")
		}
	}
}

func TestTransferNeighbors_ParentStationConnectsToItsOwnChildren(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()

	// R20 is a bare parent station (Union Square's N/Q/R/W platforms).
	// Arriving here via a cross-line transfer must be able to reach its
	// own child platforms for free, or the transfer is a dead end.
	transfers, err := transferNeighbors(ctx, conn, "R20")
	if err != nil {
		t.Fatalf("transferNeighbors: %v", err)
	}

	found := false
	for _, tr := range transfers {
		if tr.StopID == "R20N" || tr.StopID == "R20S" {
			found = true
			if tr.Cost != 0 {
				t.Errorf("expected 0 cost from a parent station to its own child platform %s, got %v", tr.StopID, tr.Cost)
			}
		}
	}
	if !found {
		t.Fatal("expected R20 to connect to at least one of its own child platforms (R20N/R20S)")
	}
}
