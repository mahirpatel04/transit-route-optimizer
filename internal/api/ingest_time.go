package api

import (
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mahirpatel04/transit-route-optimizer/internal/db"
)

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
