package api

import (
	"net/http"

	"github.com/jackc/pgx/v5"
)

func NewMux(conn *pgx.Conn) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ingest-time", handleIngestTime(conn))
	mux.HandleFunc("GET /routes", handleRoutes(conn))
	mux.HandleFunc("GET /stops/near", handleStopsNear(conn))
	return mux
}
