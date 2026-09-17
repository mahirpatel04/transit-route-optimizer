package api

import (
	"log"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/mahirpatel04/transit-route-optimizer/internal/db"
)

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
