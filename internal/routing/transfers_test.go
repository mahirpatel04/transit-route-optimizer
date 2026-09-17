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

func TestTransferNeighbors_SameStopNameDifferentParentCosts180s(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()

	// 127 and 902 are different lines' parent stations, both named
	// "Times Sq-42 St".
	transfers, err := transferNeighbors(ctx, conn, "127")
	if err != nil {
		t.Fatalf("transferNeighbors: %v", err)
	}

	found := false
	for _, tr := range transfers {
		if tr.StopID == "902" {
			found = true
			if tr.Cost != 180*time.Second {
				t.Errorf("expected 180s cross-line transfer cost, got %v", tr.Cost)
			}
		}
	}
	if !found {
		t.Fatal("expected 902 to appear as a same-stop_name cross-line transfer from 127")
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
