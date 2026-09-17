package api

import (
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/mahirpatel04/transit-route-optimizer/internal/geocode"
)

func NewMux(conn *pgx.Conn, geocoder geocode.Geocoder) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ingest-time", handleIngestTime(conn))
	mux.HandleFunc("GET /routes", handleRoutes(conn))
	mux.HandleFunc("GET /stops/near", handleStopsNear(conn))
	mux.HandleFunc("GET /route", handleRoute(conn, geocoder))
	return mux
}
