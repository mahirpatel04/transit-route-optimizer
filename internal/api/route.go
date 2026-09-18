package api

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mahirpatel04/transit-route-optimizer/internal/db"
	"github.com/mahirpatel04/transit-route-optimizer/internal/geo"
	"github.com/mahirpatel04/transit-route-optimizer/internal/geocode"
	"github.com/mahirpatel04/transit-route-optimizer/internal/gtfs"
	"github.com/mahirpatel04/transit-route-optimizer/internal/routing"
)

// originCandidateCount is always 1: the search starts from a single fixed
// platform, the one nearest the geocoded origin.
const originCandidateCount = 1

// destinationCandidateCount is how many of the nearest destination platforms
// are offered to the search as acceptable endpoints when the "optimize"
// strategy is requested (?optimize=true). This does NOT multiply the number
// of A* searches — routing.FindRouteMultiTarget evaluates all of them in one
// search (a multi-goal heuristic), so raising this doesn't reintroduce the
// per-candidate-pair timeout that motivated dropping candidateStopCount to 1
// (see git history on internal/api/route.go).
const destinationCandidateCount = 3

// nearestOnlyCandidateCount is the default (non-optimized) strategy: only
// ever consider the single nearest platform to the destination, minimizing
// the walk at that end even if a slightly farther platform would give a
// faster overall trip.
const nearestOnlyCandidateCount = 1

// walkDuration converts a straight-line distance into a walking time at
// the same pedestrian pace used for in-graph cross-complex transfers, so
// the address-to-platform legs at each end of a trip are priced
// consistently with the transfers inside it.
func walkDuration(meters float64) time.Duration {
	return time.Duration(meters/routing.PedestrianSpeedMetersPerSecond) * time.Second
}

type legResult struct {
	Kind         string `json:"kind"`
	RouteID      string `json:"route_id"`
	FromStopID   string `json:"from_stop_id"`
	FromStopName string `json:"from_stop_name"`
	ToStopID     string `json:"to_stop_id"`
	ToStopName   string `json:"to_stop_name"`
	DepartAt     string `json:"depart_at"`
	ArriveAt     string `json:"arrive_at"`
}

type routeResponse struct {
	TotalTimeSeconds float64     `json:"total_time_seconds"`
	Legs             []legResult `json:"legs"`
}

func handleRoute(conn *pgx.Conn, geocoder geocode.Geocoder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fromAddr := strings.TrimSpace(r.URL.Query().Get("from"))
		toAddr := strings.TrimSpace(r.URL.Query().Get("to"))
		if fromAddr == "" || toAddr == "" {
			writeError(w, http.StatusBadRequest, "'from' and 'to' query parameters are required")
			return
		}

		departAt := time.Now()
		if raw := r.URL.Query().Get("depart_at"); raw != "" {
			parsed, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				writeError(w, http.StatusBadRequest, "'depart_at' must be an RFC3339 timestamp")
				return
			}
			departAt = parsed
		}

		ctx := r.Context()

		fromLat, fromLon, err := geocoder.Geocode(ctx, fromAddr)
		if err != nil {
			if strings.Contains(err.Error(), "no results") {
				writeError(w, http.StatusBadRequest, "could not find 'from' address: "+err.Error())
				return
			}
			log.Printf("route: %v", err)
			writeError(w, http.StatusInternalServerError, "geocoding service unavailable")
			return
		}
		toLat, toLon, err := geocoder.Geocode(ctx, toAddr)
		if err != nil {
			if strings.Contains(err.Error(), "no results") {
				writeError(w, http.StatusBadRequest, "could not find 'to' address: "+err.Error())
				return
			}
			log.Printf("route: %v", err)
			writeError(w, http.StatusInternalServerError, "geocoding service unavailable")
			return
		}

		stops, err := db.GetAllStops(ctx, conn)
		if err != nil {
			log.Printf("route: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		stopNameByID := make(map[string]string, len(stops))
		for _, s := range stops {
			stopNameByID[s.StopId] = s.StopName
		}

		optimize := r.URL.Query().Get("optimize") == "true"
		toCandidateCount := nearestOnlyCandidateCount
		if optimize {
			toCandidateCount = destinationCandidateCount
		}

		fromCandidates := routing.NearestStops(stops, fromLat, fromLon, originCandidateCount)
		toCandidates := routing.NearestStops(stops, toLat, toLon, toCandidateCount)
		if len(fromCandidates) == 0 || len(toCandidates) == 0 {
			writeError(w, http.StatusNotFound, "no route found")
			return
		}
		fromStop := fromCandidates[0]

		toStopIDs := make([]string, len(toCandidates))
		toStopByID := make(map[string]gtfs.Stop, len(toCandidates))
		for i, s := range toCandidates {
			toStopIDs[i] = s.StopId
			toStopByID[s.StopId] = s
		}

		initialWalk := walkDuration(geo.Haversine(fromLat, fromLon, fromStop.Lat, fromStop.Lon))
		route, err := routing.FindRouteMultiTarget(ctx, conn, fromStop.StopId, toStopIDs, departAt.Add(initialWalk))
		if err != nil {
			if !strings.Contains(err.Error(), "no route found") {
				log.Printf("route: %v", err)
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
			writeError(w, http.StatusNotFound, "no route found")
			return
		}

		// toStop is whichever destination candidate the search actually
		// reached — the fastest one, not necessarily the nearest. An empty
		// route (no legs) means the origin platform was itself already one
		// of the destination candidates, so it's also the arrival point.
		toStop := fromStop
		if len(route.Legs) > 0 {
			toStop = toStopByID[route.Legs[len(route.Legs)-1].ToStopID]
		}
		finalWalk := walkDuration(geo.Haversine(toLat, toLon, toStop.Lat, toStop.Lon))
		total := initialWalk + route.TotalTime + finalWalk

		firstDepartAt := departAt
		legs := make([]legResult, 0, len(route.Legs)+2)
		legs = append(legs, legResult{
			Kind:       "walk",
			ToStopID:   fromStop.StopId,
			ToStopName: fromStop.StopName,
			DepartAt:   firstDepartAt.Format(time.RFC3339),
			ArriveAt:   firstDepartAt.Add(initialWalk).Format(time.RFC3339),
		})
		for _, leg := range route.Legs {
			legs = append(legs, legResult{
				Kind:         string(leg.Kind),
				RouteID:      leg.RouteID,
				FromStopID:   leg.FromStopID,
				FromStopName: stopNameByID[leg.FromStopID],
				ToStopID:     leg.ToStopID,
				ToStopName:   stopNameByID[leg.ToStopID],
				DepartAt:     leg.DepartAt.Format(time.RFC3339),
				ArriveAt:     leg.ArriveAt.Format(time.RFC3339),
			})
		}
		lastArriveAt := firstDepartAt.Add(initialWalk).Add(route.TotalTime)
		legs = append(legs, legResult{
			Kind:         "walk",
			FromStopID:   toStop.StopId,
			FromStopName: toStop.StopName,
			ToStopName:   toAddr,
			DepartAt:     lastArriveAt.Format(time.RFC3339),
			ArriveAt:     lastArriveAt.Add(finalWalk).Format(time.RFC3339),
		})

		writeJSON(w, http.StatusOK, routeResponse{
			TotalTimeSeconds: total.Seconds(),
			Legs:             legs,
		})
	}
}
