package routing

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mahirpatel04/transit-route-optimizer/internal/geo"
)

// PedestrianSpeedMetersPerSecond is an average walking pace (~5 km/h), used
// to price a cross-complex transfer by actual distance rather than a flat
// cost — unlike maxSpeedMetersPerSecond in routing.go, this doesn't need to
// be a conservative bound, since transfer cost isn't used as a search
// heuristic.
const PedestrianSpeedMetersPerSecond = 1.4

// crossComplexTransferRadiusMeters bounds how far apart two station
// complexes can be and still count as a walkable transfer. Complexes this
// close together in real life are connected by a passageway or a short
// street-level walk regardless of whether their GTFS stop_names match —
// MTA names the same physical complex differently per line often enough
// (e.g. "Court Sq" vs "Court Sq-23 St") that requiring an exact match missed
// real transfers.
const crossComplexTransferRadiusMeters = 250

type transfer struct {
	StopID string
	Cost   time.Duration
}

// transferNeighbors returns every stop reachable from stopID by walking
// within a station complex: free between platforms sharing the same
// parent_station, and a distance-priced walk to any other station complex
// within crossComplexTransferRadiusMeters.
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

	// s2 is restricted to bare parent stations (parent_station IS NULL) so
	// each candidate complex is represented by exactly one row/coordinate,
	// regardless of stop_name — MTA doesn't always name a shared physical
	// complex identically across lines (e.g. "Court Sq" vs "Court Sq-23
	// St"), so matching by name alone silently drops real transfers.
	// COALESCE handles stopID itself being a parent station (parent_station
	// IS NULL, e.g. "127") by comparing against its own stop_id instead —
	// so this works whether stopID is a child platform or a parent station.
	crossComplexRows, err := conn.Query(ctx, `
		SELECT s2.stop_id, s1.lat, s1.lon, s2.lat, s2.lon
		FROM stops s1
		JOIN stops s2 ON s2.parent_station IS NULL
		         AND s2.stop_id != COALESCE(s1.parent_station, s1.stop_id)
		WHERE s1.stop_id = $1
	`, stopID)
	if err != nil {
		return nil, fmt.Errorf("failed to query cross-complex transfers for %s: %w", stopID, err)
	}
	defer crossComplexRows.Close()
	for crossComplexRows.Next() {
		var otherParentStation string
		var s1Lat, s1Lon, s2Lat, s2Lon float64
		if err := crossComplexRows.Scan(&otherParentStation, &s1Lat, &s1Lon, &s2Lat, &s2Lon); err != nil {
			return nil, fmt.Errorf("failed to scan cross-complex transfer: %w", err)
		}
		meters := geo.Haversine(s1Lat, s1Lon, s2Lat, s2Lon)
		if meters > crossComplexTransferRadiusMeters {
			continue // too far apart to be a realistic walking transfer
		}
		cost := time.Duration(meters/PedestrianSpeedMetersPerSecond) * time.Second
		transfers = append(transfers, transfer{StopID: otherParentStation, Cost: cost})
	}
	if err := crossComplexRows.Err(); err != nil {
		return nil, fmt.Errorf("failed reading cross-complex transfers: %w", err)
	}

	return transfers, nil
}
