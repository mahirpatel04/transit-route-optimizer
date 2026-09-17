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
