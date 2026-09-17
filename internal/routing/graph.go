package routing

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type edgeKind string

const (
	edgeKindWalk edgeKind = "walk"
	edgeKindRide edgeKind = "ride"
)

type edge struct {
	ToStopID string
	RouteID  string // empty for walk edges
	Kind     edgeKind
	DepartAt time.Time
	ArriveAt time.Time
}

// serviceWindow pairs a calendar date with the GTFS time-of-day offset to
// query stop_times against for that date.
type serviceWindow struct {
	date         time.Time
	gtfsTimeFrom time.Duration
}

// serviceWindowsFor returns the two service-day windows that must be
// checked for real time `at`: the literal calendar day, and the previous
// calendar day's post-midnight (>=24:00:00) continuation.
func serviceWindowsFor(at time.Time) [2]serviceWindow {
	dayStart := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC)
	timeOfDay := at.Sub(dayStart)

	return [2]serviceWindow{
		{date: dayStart, gtfsTimeFrom: timeOfDay},
		{date: dayStart.AddDate(0, 0, -1), gtfsTimeFrom: timeOfDay + 24*time.Hour},
	}
}

// expand returns every edge reachable from stopID starting at real time
// `at`: one ride edge per route (earliest trip after `at`, to its next
// stop), plus transfer edges within the station complex.
func expand(ctx context.Context, conn *pgx.Conn, stopID string, at time.Time) ([]edge, error) {
	var edges []edge

	for _, w := range serviceWindowsFor(at) {
		serviceIDs, err := activeServiceIDs(ctx, conn, w.date)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve active services for %s: %w", w.date, err)
		}
		if len(serviceIDs) == 0 {
			continue
		}

		candidates, err := nextDeparturesByRoute(ctx, conn, stopID, w.gtfsTimeFrom, serviceIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to find next departures from %s: %w", stopID, err)
		}

		dayStart := time.Date(w.date.Year(), w.date.Month(), w.date.Day(), 0, 0, 0, 0, time.UTC)

		for _, c := range candidates {
			next, ok, err := nextStopOnTrip(ctx, conn, c.TripID, c.StopSequence)
			if err != nil {
				return nil, fmt.Errorf("failed to find next stop for trip %s: %w", c.TripID, err)
			}
			if !ok {
				continue // this trip ends here, no onward edge
			}
			edges = append(edges, edge{
				ToStopID: next.StopID,
				RouteID:  c.RouteID,
				Kind:     edgeKindRide,
				DepartAt: dayStart.Add(c.DepartureTime),
				ArriveAt: dayStart.Add(next.ArrivalTime),
			})
		}
	}

	transfers, err := transferNeighbors(ctx, conn, stopID)
	if err != nil {
		return nil, fmt.Errorf("failed to find transfers from %s: %w", stopID, err)
	}
	for _, tr := range transfers {
		edges = append(edges, edge{
			ToStopID: tr.StopID,
			Kind:     edgeKindWalk,
			DepartAt: at,
			ArriveAt: at.Add(tr.Cost),
		})
	}

	return edges, nil
}
