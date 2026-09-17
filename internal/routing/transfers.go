package routing

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

const crossLineTransferCost = 180 * time.Second

type transfer struct {
	StopID string
	Cost   time.Duration
}

// transferNeighbors returns every stop reachable from stopID by walking
// within a station complex: free between platforms sharing the same
// parent_station, and a fixed cost between different lines' parent
// stations that share the same stop_name (the same physical complex).
func transferNeighbors(ctx context.Context, conn *pgx.Conn, stopID string) ([]transfer, error) {
	var transfers []transfer

	sameParentRows, err := conn.Query(ctx, `
		SELECT s2.stop_id
		FROM stops s1
		JOIN stops s2 ON s2.parent_station = s1.parent_station AND s2.stop_id != s1.stop_id
		WHERE s1.stop_id = $1 AND s1.parent_station IS NOT NULL
	`, stopID)
	// Deliberately no fallback here for when stopID is itself a parent
	// station (e.g. "127", which has no siblings under this query) — a
	// parent station's own child platforms are not "the same stop," so it
	// correctly returns no same-parent_station transfers in that case.
	if err != nil {
		return nil, fmt.Errorf("failed to query same-parent_station transfers for %s: %w", stopID, err)
	}
	for sameParentRows.Next() {
		var otherStopID string
		if err := sameParentRows.Scan(&otherStopID); err != nil {
			sameParentRows.Close()
			return nil, fmt.Errorf("failed to scan same-parent_station transfer: %w", err)
		}
		transfers = append(transfers, transfer{StopID: otherStopID, Cost: 0})
	}
	if err := sameParentRows.Err(); err != nil {
		sameParentRows.Close()
		return nil, fmt.Errorf("failed reading same-parent_station transfers: %w", err)
	}
	sameParentRows.Close()

	// COALESCE handles stopID itself being a parent station (parent_station
	// IS NULL, e.g. "127") by comparing against its own stop_id instead —
	// so this works whether stopID is a child platform or a parent station.
	crossLineRows, err := conn.Query(ctx, `
		SELECT DISTINCT s2.parent_station
		FROM stops s1
		JOIN stops s2 ON s2.stop_name = s1.stop_name
		         AND s2.parent_station IS NOT NULL
		         AND s2.parent_station != COALESCE(s1.parent_station, s1.stop_id)
		WHERE s1.stop_id = $1
	`, stopID)
	if err != nil {
		return nil, fmt.Errorf("failed to query cross-line transfers for %s: %w", stopID, err)
	}
	defer crossLineRows.Close()
	for crossLineRows.Next() {
		var otherParentStation string
		if err := crossLineRows.Scan(&otherParentStation); err != nil {
			return nil, fmt.Errorf("failed to scan cross-line transfer: %w", err)
		}
		transfers = append(transfers, transfer{StopID: otherParentStation, Cost: crossLineTransferCost})
	}
	if err := crossLineRows.Err(); err != nil {
		return nil, fmt.Errorf("failed reading cross-line transfers: %w", err)
	}

	return transfers, nil
}
