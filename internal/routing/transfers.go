package routing

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mahirpatel04/transit-route-optimizer/internal/geo"
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
		JOIN stops s2 ON s2.parent_station = COALESCE(s1.parent_station, s1.stop_id) AND s2.stop_id != s1.stop_id
		WHERE s1.stop_id = $1
	`, stopID)
	// COALESCE handles stopID itself being a parent station (parent_station
	// IS NULL, e.g. "R20") by comparing against its own stop_id instead, so
	// a parent station correctly connects to its own child platforms for
	// free (a cross-line transfer that lands on a bare parent station must
	// be able to board any of that complex's platforms) — this is a no-op
	// for child-platform callers, which still only match same-parent
	// siblings as before.
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
		SELECT DISTINCT s2.parent_station, s1.lat, s1.lon, s2.lat, s2.lon
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
		var s1Lat, s1Lon, s2Lat, s2Lon float64
		if err := crossLineRows.Scan(&otherParentStation, &s1Lat, &s1Lon, &s2Lat, &s2Lon); err != nil {
			return nil, fmt.Errorf("failed to scan cross-line transfer: %w", err)
		}
		if geo.Haversine(s1Lat, s1Lon, s2Lat, s2Lon) > 400 {
			continue // different physical complex despite sharing a stop_name
		}
		transfers = append(transfers, transfer{StopID: otherParentStation, Cost: crossLineTransferCost})
	}
	if err := crossLineRows.Err(); err != nil {
		return nil, fmt.Errorf("failed reading cross-line transfers: %w", err)
	}

	return transfers, nil
}
