package api

import (
	"log"
	"net/http"
	"sort"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/mahirpatel04/transit-route-optimizer/internal/db"
	"github.com/mahirpatel04/transit-route-optimizer/internal/geo"
)

const (
	defaultStopsLimit = 10
	maxStopsLimit      = 50
)

type stopNearResult struct {
	StopId         string  `json:"stop_id"`
	StopName       string  `json:"stop_name"`
	Lat            float64 `json:"lat"`
	Lon            float64 `json:"lon"`
	DistanceMeters float64 `json:"distance_meters"`
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
		if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 && l <= maxStopsLimit {
			limit = l
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
