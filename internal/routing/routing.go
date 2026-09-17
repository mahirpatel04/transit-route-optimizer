package routing

import (
	"container/heap"
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mahirpatel04/transit-route-optimizer/internal/db"
	"github.com/mahirpatel04/transit-route-optimizer/internal/geo"
)

type LegKind string

const (
	LegKindWalk LegKind = "walk"
	LegKindRide LegKind = "ride"
)

type Leg struct {
	Kind       LegKind
	RouteID    string
	FromStopID string
	ToStopID   string
	DepartAt   time.Time
	ArriveAt   time.Time
}

type Route struct {
	Legs      []Leg
	TotalTime time.Duration
}

// cameFrom records how a stop was reached during the search, so the final
// path can be reconstructed by walking these breadcrumbs backward from the
// destination once it's popped.
type cameFrom struct {
	stopID   string
	kind     LegKind
	routeID  string
	departAt time.Time
	arriveAt time.Time
}

// maxSpeedMetersPerSecond is a deliberately fast, conservative upper bound
// (30 mph) on how quickly a rider could possibly cover ground, used only to
// keep the A* heuristic admissible (it must never overestimate remaining
// time, or the search could discard the true best route).
const maxSpeedMetersPerSecond = 13.4

// searchNode is one entry in the priority queue: a stop reached at a given
// time, with enough breadcrumb info to reconstruct the path once popped at
// the destination.
type searchNode struct {
	stopID     string
	arriveAt   time.Time
	fScore     time.Time // arriveAt + heuristic, used for ordering
	fromStopID string
	viaKind    LegKind
	viaRoute   string
	viaDepart  time.Time
	index      int // heap.Interface bookkeeping
}

type priorityQueue []*searchNode

func (pq priorityQueue) Len() int { return len(pq) }
func (pq priorityQueue) Less(i, j int) bool {
	return pq[i].fScore.Before(pq[j].fScore)
}
func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}
func (pq *priorityQueue) Push(x any) {
	n := x.(*searchNode)
	n.index = len(*pq)
	*pq = append(*pq, n)
}
func (pq *priorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*pq = old[:n-1]
	return item
}

// FindRoute runs a time-dependent A* search from fromStopID to toStopID,
// departing at or after departAt, and returns the fastest path as an
// ordered list of legs.
func FindRoute(ctx context.Context, conn *pgx.Conn, fromStopID, toStopID string, departAt time.Time) (Route, error) {
	if fromStopID == toStopID {
		return Route{}, nil
	}

	stops, err := db.GetAllStops(ctx, conn)
	if err != nil {
		return Route{}, fmt.Errorf("failed to load stop coordinates: %w", err)
	}
	coords := make(map[string]struct{ lat, lon float64 }, len(stops))
	for _, s := range stops {
		coords[s.StopId] = struct{ lat, lon float64 }{s.Lat, s.Lon}
	}

	destCoord, ok := coords[toStopID]
	if !ok {
		return Route{}, fmt.Errorf("unknown destination stop %s", toStopID)
	}

	heuristic := func(stopID string) time.Duration {
		c, ok := coords[stopID]
		if !ok {
			return 0
		}
		meters := geo.Haversine(c.lat, c.lon, destCoord.lat, destCoord.lon)
		return time.Duration(meters/maxSpeedMetersPerSecond) * time.Second
	}

	best := map[string]time.Time{fromStopID: departAt}
	predecessor := map[string]cameFrom{}

	pq := &priorityQueue{}
	heap.Init(pq)
	heap.Push(pq, &searchNode{
		stopID:   fromStopID,
		arriveAt: departAt,
		fScore:   departAt.Add(heuristic(fromStopID)),
	})

	visited := map[string]bool{}

	for pq.Len() > 0 {
		current := heap.Pop(pq).(*searchNode)
		if visited[current.stopID] {
			continue
		}
		visited[current.stopID] = true

		if current.stopID == toStopID {
			return reconstructRoute(fromStopID, toStopID, predecessor, current.arriveAt), nil
		}

		edges, err := expand(ctx, conn, current.stopID, current.arriveAt)
		if err != nil {
			return Route{}, fmt.Errorf("failed to expand stop %s: %w", current.stopID, err)
		}

		for _, e := range edges {
			knownBest, seen := best[e.ToStopID]
			if seen && !e.ArriveAt.Before(knownBest) {
				continue
			}
			best[e.ToStopID] = e.ArriveAt
			predecessor[e.ToStopID] = cameFrom{
				stopID:   current.stopID,
				kind:     LegKind(e.Kind),
				routeID:  e.RouteID,
				departAt: e.DepartAt,
				arriveAt: e.ArriveAt,
			}
			heap.Push(pq, &searchNode{
				stopID:   e.ToStopID,
				arriveAt: e.ArriveAt,
				fScore:   e.ArriveAt.Add(heuristic(e.ToStopID)),
			})
		}
	}

	return Route{}, fmt.Errorf("no route found from %s to %s", fromStopID, toStopID)
}

func reconstructRoute(fromStopID, toStopID string, predecessor map[string]cameFrom, finalArrival time.Time) Route {
	var legs []Leg
	stopID := toStopID
	for stopID != fromStopID {
		p, ok := predecessor[stopID]
		if !ok {
			break
		}
		legs = append([]Leg{{
			Kind:       p.kind,
			RouteID:    p.routeID,
			FromStopID: p.stopID,
			ToStopID:   stopID,
			DepartAt:   p.departAt,
			ArriveAt:   p.arriveAt,
		}}, legs...)
		stopID = p.stopID
	}

	var totalTime time.Duration
	if len(legs) > 0 {
		totalTime = finalArrival.Sub(legs[0].DepartAt)
	}

	return Route{Legs: legs, TotalTime: totalTime}
}
