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

// Each additional candidate multiplies the number of full A* searches
// (fromCandidates × toCandidates), and each search can issue hundreds of
// sequential DB round trips — see internal/routing/graph.go's expand(). At 3
// candidates (9 searches) this blew past the Lambda's request timeout; 1
// keeps a single request to a single search.
const candidateStopCount = 1

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

		fromCandidates := routing.NearestStops(stops, fromLat, fromLon, candidateStopCount)
		toCandidates := routing.NearestStops(stops, toLat, toLon, candidateStopCount)

		var best routing.Route
		var bestFromStop, bestToStop gtfs.Stop
		var bestInitialWalk, bestFinalWalk time.Duration
		var bestTotal time.Duration
		found := false
	candidatePairLoop:
		for _, fromStop := range fromCandidates {
			for _, toStop := range toCandidates {
				if ctx.Err() != nil {
					break candidatePairLoop
				}
				if fromStop.StopId == toStop.StopId {
					continue
				}
				initialWalk := walkDuration(geo.Haversine(fromLat, fromLon, fromStop.Lat, fromStop.Lon))
				route, err := routing.FindRoute(ctx, conn, fromStop.StopId, toStop.StopId, departAt.Add(initialWalk))
				if err != nil {
					if !strings.Contains(err.Error(), "no route found") {
						log.Printf("route: %v", err)
					}
					continue // this candidate pair has no route; try the next
				}
				finalWalk := walkDuration(geo.Haversine(toLat, toLon, toStop.Lat, toStop.Lon))
				total := initialWalk + route.TotalTime + finalWalk
				if !found || total < bestTotal {
					best = route
					bestFromStop = fromStop
					bestToStop = toStop
					bestInitialWalk = initialWalk
					bestFinalWalk = finalWalk
					bestTotal = total
					found = true
				}
			}
		}

		if !found {
			writeError(w, http.StatusNotFound, "no route found")
			return
		}

		firstDepartAt := departAt
		legs := make([]legResult, 0, len(best.Legs)+2)
		legs = append(legs, legResult{
			Kind:       "walk",
			ToStopID:   bestFromStop.StopId,
			ToStopName: bestFromStop.StopName,
			DepartAt:   firstDepartAt.Format(time.RFC3339),
			ArriveAt:   firstDepartAt.Add(bestInitialWalk).Format(time.RFC3339),
		})
		for _, leg := range best.Legs {
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
		lastArriveAt := best.Legs[len(best.Legs)-1].ArriveAt
		legs = append(legs, legResult{
			Kind:         "walk",
			FromStopID:   bestToStop.StopId,
			FromStopName: bestToStop.StopName,
			ToStopName:   toAddr,
			DepartAt:     lastArriveAt.Format(time.RFC3339),
			ArriveAt:     lastArriveAt.Add(bestFinalWalk).Format(time.RFC3339),
		})

		writeJSON(w, http.StatusOK, routeResponse{
			TotalTimeSeconds: bestTotal.Seconds(),
			Legs:             legs,
		})
	}
}
