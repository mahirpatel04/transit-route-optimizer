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
	"github.com/mahirpatel04/transit-route-optimizer/internal/gtfs"
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

	srv := httptest.NewServer(NewMux(conn))
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

func TestHandleRoutes(t *testing.T) {
	srv, conn := testServer(t)
	ctx := context.Background()

	routes := []gtfs.Route{{RouteId: "A", RouteName: "8th Avenue Express", RouteType: 1}}
	if _, err := db.InsertRoutes(ctx, conn, routes); err != nil {
		t.Fatalf("InsertRoutes: %v", err)
	}

	resp, err := http.Get(srv.URL + "/routes")
	if err != nil {
		t.Fatalf("GET /routes: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got status %d, want 200", resp.StatusCode)
	}

	var got []gtfs.Route
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].RouteId != "A" {
		t.Errorf("got %+v, want one route with id A", got)
	}
}

func TestHandleStopsNear_MissingLatLon(t *testing.T) {
	srv, _ := testServer(t)

	resp, err := http.Get(srv.URL + "/stops/near")
	if err != nil {
		t.Fatalf("GET /stops/near: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("got status %d, want 400", resp.StatusCode)
	}
}

func TestHandleStopsNear_ReturnsSortedByDistance(t *testing.T) {
	srv, conn := testServer(t)
	ctx := context.Background()

	stops := []gtfs.Stop{
		{StopId: "far", StopName: "Far Stop", Lat: 40.8, Lon: -74.1},
		{StopId: "near", StopName: "Near Stop", Lat: 40.7581, Lon: -73.9856},
	}
	if _, err := db.InsertStops(ctx, conn, stops); err != nil {
		t.Fatalf("InsertStops: %v", err)
	}

	resp, err := http.Get(srv.URL + "/stops/near?lat=40.7580&lon=-73.9855&limit=2")
	if err != nil {
		t.Fatalf("GET /stops/near: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got status %d, want 200", resp.StatusCode)
	}

	var got []stopNearResult
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d results, want 2", len(got))
	}
	if got[0].StopId != "near" {
		t.Errorf("got nearest stop %q, want %q", got[0].StopId, "near")
	}
}
