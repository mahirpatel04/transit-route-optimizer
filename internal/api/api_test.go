package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/mahirpatel04/transit-route-optimizer/internal/db"
)

func testServer(t *testing.T) (*httptest.Server, *pgx.Conn) {
	t.Helper()
	godotenv.Load("../../.env")
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; run `make up` for local Postgres")
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := db.TruncateAll(ctx, conn); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	srv := httptest.NewServer(NewMux(conn, nil))
	t.Cleanup(func() {
		srv.Close()
		conn.Close(ctx)
	})
	return srv, conn
}

func TestHandleIngestTime(t *testing.T) {
	srv, conn := testServer(t)
	ctx := context.Background()

	want := time.Now().UTC().Truncate(time.Second)
	if err := db.SetLastFetchTime(ctx, conn, want); err != nil {
		t.Fatalf("SetLastFetchTime: %v", err)
	}

	resp, err := http.Get(srv.URL + "/ingest-time")
	if err != nil {
		t.Fatalf("GET /ingest-time: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got status %d, want 200", resp.StatusCode)
	}

	var body struct {
		LastFetchTime time.Time `json:"last_fetch_time"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.LastFetchTime.Equal(want) {
		t.Errorf("got %v, want %v", body.LastFetchTime, want)
	}
}
