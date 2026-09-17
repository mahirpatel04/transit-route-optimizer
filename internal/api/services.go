package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mahirpatel04/transit-route-optimizer/internal/db"
	"github.com/mahirpatel04/transit-route-optimizer/internal/geo"
)

const (
	defaultStopsLimit = 10
	maxStopsLimit     = 50
)

type stopNearResult struct {
	StopId         string  `json:"stop_id"`
	StopName       string  `json:"stop_name"`
	Lat            float64 `json:"lat"`
	Lon            float64 `json:"lon"`
	DistanceMeters float64 `json:"distance_meters"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func handleIngestTime(conn *pgx.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t, err := db.GetLastFetchTime(r.Context(), conn)
		if err != nil {
			log.Printf("ingest-time: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"last_fetch_time": t.Format(time.RFC3339),
		})
	}
}

func handleRoutes(conn *pgx.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		routes, err := db.GetRoutes(r.Context(), conn)
		if err != nil {
			log.Printf("routes: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, routes)
	}
}

func handleStopsNear(conn *pgx.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lat, latErr := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
		lon, lonErr := strconv.ParseFloat(r.URL.Query().Get("lon"), 64)
		if latErr != nil || lonErr != nil {
			writeError(w, http.StatusBadRequest, "invalid or missing lat/lon")
			return
		}

		limit := defaultStopsLimit
		if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
			limit = min(l, maxStopsLimit)
		}

		stops, err := db.GetAllStops(r.Context(), conn)
		if err != nil {
			log.Printf("stops/near: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		results := make([]stopNearResult, len(stops))
		for i, s := range stops {
			results[i] = stopNearResult{
				StopId:         s.StopId,
				StopName:       s.StopName,
				Lat:            s.Lat,
				Lon:            s.Lon,
				DistanceMeters: geo.Haversine(lat, lon, s.Lat, s.Lon),
			}
		}
		sort.Slice(results, func(i, j int) bool {
			return results[i].DistanceMeters < results[j].DistanceMeters
		})
		if len(results) > limit {
			results = results[:limit]
		}

		writeJSON(w, http.StatusOK, results)
	}
}
