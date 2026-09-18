package routing

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mahirpatel04/transit-route-optimizer/internal/geo"
)

// PedestrianSpeedMetersPerSecond is an average walking pace (~5 km/h) used
// to price cross-complex transfer walks by actual distance.
const PedestrianSpeedMetersPerSecond = 1.4

// crossComplexTransferRadiusMeters bounds how far apart two station
// complexes can be and still count as a walkable transfer — matched by
// proximity, not GTFS stop_name, since MTA names the same physical complex
// differently across lines (e.g. "Court Sq" vs "Court Sq-23 St").
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
	// COALESCE lets a bare parent station (parent_station IS NULL) match
	// against its own stop_id, so a transfer landing on the parent can still
	// board any child platform.
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

	// s2 is restricted to bare parent stations so each candidate complex is
	// one row/coordinate; COALESCE lets stopID itself be a parent station.
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
