package routing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type candidateTrip struct {
	TripID        string
	RouteID       string
	StopSequence  int
	DepartureTime time.Duration
	NextStopID    string
	NextArrival   time.Duration
	HasNextStop   bool
}

// nextDeparturesByRoute returns, for each distinct route serving stopID, the
// earliest trip departing at or after afterGTFSTime among the given active
// serviceIDs, along with that trip's next stop (via a LATERAL join, so no
// separate per-candidate round trip is needed to find it). One candidate per
// route — a later trip on the same route can never be a better edge than the
// earliest one, since NYC subway trips on the same route+direction visit
// stops in the same order.
func nextDeparturesByRoute(ctx context.Context, conn *pgx.Conn, stopID string, afterGTFSTime time.Duration, serviceIDs []string) ([]candidateTrip, error) {
	if len(serviceIDs) == 0 {
		return nil, nil
	}

	rows, err := conn.Query(ctx, `
		SELECT DISTINCT ON (t.route_id)
			st.trip_id, t.route_id, st.stop_sequence, st.departure_time,
			nxt.stop_id, nxt.arrival_time, (nxt.stop_id IS NOT NULL) AS has_next
		FROM stop_times st
		JOIN trips t ON t.trip_id = st.trip_id
		LEFT JOIN LATERAL (
			SELECT stop_id, arrival_time
			FROM stop_times
			WHERE trip_id = st.trip_id AND stop_sequence > st.stop_sequence
			ORDER BY stop_sequence ASC
			LIMIT 1
		) nxt ON true
		WHERE st.stop_id = $1
		  AND st.departure_time >= $2
		  AND t.service_id = ANY($3)
		ORDER BY t.route_id, st.departure_time ASC
	`, stopID, afterGTFSTime, serviceIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query next departures for stop %s: %w", stopID, err)
	}
	defer rows.Close()

	var candidates []candidateTrip
	for rows.Next() {
		var c candidateTrip
		var nextStopID *string
		var nextArrival *time.Duration
		if err := rows.Scan(&c.TripID, &c.RouteID, &c.StopSequence, &c.DepartureTime, &nextStopID, &nextArrival, &c.HasNextStop); err != nil {
			return nil, fmt.Errorf("failed to scan candidate trip: %w", err)
		}
		if c.HasNextStop {
			c.NextStopID = *nextStopID
			c.NextArrival = *nextArrival
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed reading candidate trips: %w", err)
	}
	return candidates, nil
}

type nextStop struct {
	StopID      string
	ArrivalTime time.Duration
}

// nextStopOnTrip returns the immediate next stop on tripID after
// afterStopSequence, or ok=false if afterStopSequence was the last stop on
// that trip (or the trip doesn't exist).
func nextStopOnTrip(ctx context.Context, conn *pgx.Conn, tripID string, afterStopSequence int) (nextStop, bool, error) {
	var ns nextStop
	err := conn.QueryRow(ctx, `
		SELECT stop_id, arrival_time
		FROM stop_times
		WHERE trip_id = $1 AND stop_sequence > $2
		ORDER BY stop_sequence ASC
		LIMIT 1
	`, tripID, afterStopSequence).Scan(&ns.StopID, &ns.ArrivalTime)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nextStop{}, false, nil
		}
		return nextStop{}, false, fmt.Errorf("failed to query next stop for trip %s: %w", tripID, err)
	}
	return ns, true, nil
}
