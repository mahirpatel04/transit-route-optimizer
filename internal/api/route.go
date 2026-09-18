package api

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mahirpatel04/transit-route-optimizer/internal/db"
	"github.com/mahirpatel04/transit-route-optimizer/internal/geocode"
	"github.com/mahirpatel04/transit-route-optimizer/internal/routing"
)

const candidateStopCount = 3

type legResult struct {
	Kind       string `json:"kind"`
	RouteID    string `json:"route_id"`
	FromStopID string `json:"from_stop_id"`
	ToStopID   string `json:"to_stop_id"`
	DepartAt   string `json:"depart_at"`
	ArriveAt   string `json:"arrive_at"`
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

		fromCandidates := routing.NearestStops(stops, fromLat, fromLon, candidateStopCount)
		toCandidates := routing.NearestStops(stops, toLat, toLon, candidateStopCount)

		var best routing.Route
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
				route, err := routing.FindRoute(ctx, conn, fromStop.StopId, toStop.StopId, departAt)
				if err != nil {
					if !strings.Contains(err.Error(), "no route found") {
						log.Printf("route: %v", err)
					}
					continue // this candidate pair has no route; try the next
				}
				if !found || route.TotalTime < best.TotalTime {
					best = route
					found = true
				}
			}
		}

		if !found {
			writeError(w, http.StatusNotFound, "no route found")
			return
		}

		legs := make([]legResult, len(best.Legs))
		for i, leg := range best.Legs {
			legs[i] = legResult{
				Kind:       string(leg.Kind),
				RouteID:    leg.RouteID,
				FromStopID: leg.FromStopID,
				ToStopID:   leg.ToStopID,
				DepartAt:   leg.DepartAt.Format(time.RFC3339),
				ArriveAt:   leg.ArriveAt.Format(time.RFC3339),
			}
		}

		writeJSON(w, http.StatusOK, routeResponse{
			TotalTimeSeconds: best.TotalTime.Seconds(),
			Legs:             legs,
		})
	}
}
