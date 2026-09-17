package api

import (
	"encoding/json"
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
