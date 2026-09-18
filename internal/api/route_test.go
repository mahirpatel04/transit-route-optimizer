package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

// fakeGeocoder resolves a fixed set of known addresses to coordinates near
// real, already-ingested NYC subway stops — route_test.go deliberately does
// not truncate the database (unlike api_test.go's testServer), since
// finding a real route requires real trips/stop_times/calendar data that
// only `make ingest-dev` provides.
type fakeGeocoder map[string]struct{ lat, lon float64 }

func (f fakeGeocoder) Geocode(ctx context.Context, address string) (float64, float64, error) {
	if v, ok := f[address]; ok {
		return v.lat, v.lon, nil
	}
	return 0, 0, errors.New("no results for address " + address)
}

func testConnForRoute(t *testing.T) *pgx.Conn {
	t.Helper()
	godotenv.Load("../../.env")
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; run `make up && make ingest-dev` for local Postgres with real ingested GTFS data")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { conn.Close(ctx) })
	return conn
}

func TestHandleRoute_MissingFromOrTo(t *testing.T) {
	conn := testConnForRoute(t)
	srv := httptest.NewServer(NewMux(conn, fakeGeocoder{}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/route?from=Times+Square")
	if err != nil {
		t.Fatalf("GET /route: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("got status %d, want 400", resp.StatusCode)
	}
}

func TestHandleRoute_GeocodeFailureReturns400(t *testing.T) {
	conn := testConnForRoute(t)
	srv := httptest.NewServer(NewMux(conn, fakeGeocoder{}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/route?from=an+address+not+in+the+fake+geocoder&to=also+not+in+it")
	if err != nil {
		t.Fatalf("GET /route: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("got status %d, want 400", resp.StatusCode)
	}
}

func TestHandleRoute_ReturnsRouteBetweenRealStops(t *testing.T) {
	conn := testConnForRoute(t)

	// These coordinates are close to (but need not exactly match) the real
	// Times Sq-42 St and Grand Central-42 St platforms already present from
	// `make ingest-dev` — NearestStops resolves each to the real nearby
	// platform regardless of the exact input point.
	geocoder := fakeGeocoder{
		"Times Square, NYC":  {40.7580, -73.9855},
		"Grand Central, NYC": {40.7527, -73.9772},
	}
	srv := httptest.NewServer(NewMux(conn, geocoder))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/route?from=Times+Square,+NYC&to=Grand+Central,+NYC")
	if err != nil {
		t.Fatalf("GET /route: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got status %d, want 200", resp.StatusCode)
	}

	var body struct {
		TotalTimeSeconds float64 `json:"total_time_seconds"`
		Legs             []struct {
			Kind         string `json:"kind"`
			FromStopName string `json:"from_stop_name"`
			ToStopName   string `json:"to_stop_name"`
		} `json:"legs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Legs) < 2 {
		t.Fatalf("expected at least the initial and final walk legs plus transit, got %d legs", len(body.Legs))
	}
	if body.TotalTimeSeconds <= 0 {
		t.Errorf("expected positive total time, got %v", body.TotalTimeSeconds)
	}

	first := body.Legs[0]
	if first.Kind != "walk" || first.ToStopName == "" {
		t.Errorf("expected a leading walk leg with a named destination stop, got %+v", first)
	}

	last := body.Legs[len(body.Legs)-1]
	if last.Kind != "walk" || last.FromStopName == "" || last.ToStopName != "Grand Central, NYC" {
		t.Errorf("expected a trailing walk leg from a named stop to the requested 'to' address, got %+v", last)
	}

	for _, leg := range body.Legs[1 : len(body.Legs)-1] {
		if leg.FromStopName == "" || leg.ToStopName == "" {
			t.Errorf("expected every mid-route leg to carry stop names, got %+v", leg)
		}
	}
}
