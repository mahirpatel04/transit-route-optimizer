# Public Read-Only API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose `/ingest-time`, `/routes`, and `/stops/near` as public, read-only, unauthenticated HTTP endpoints on the existing ingest Lambda, without disturbing its cron-triggered ingest behavior.

**Architecture:** One Lambda, two trigger types. `cmd/ingest/main.go`'s Lambda handler inspects the raw event, dispatches EventBridge cron events to the existing `run(ctx)` ingest path unchanged, and routes anything else (Function URL HTTP requests) through a `net/http.ServeMux` in a new `internal/api` package, adapted via `httpadapter`. New `internal/geo` and `internal/db` additions supply the data the handlers need.

**Tech Stack:** Go 1.27, `net/http` (stdlib, 1.22+ method/path-variable routing), `github.com/awslabs/aws-lambda-go-api-proxy/httpadapter`, `github.com/aws/aws-lambda-go/events`, `pgx/v5`, AWS CDK (Go) for infra.

**Spec:** `docs/superpowers/specs/2026-09-17-public-api-design.md`

## Global Constraints

- One shared Lambda function — do not create a second Lambda; extend the existing `IngestFunction`.
- No authentication and no API Gateway — public access via a Lambda Function URL with `AuthType: NONE`.
- `ReservedConcurrentExecutions: 5` on the Lambda (caps concurrency across both the cron and API paths).
- Exactly 3 endpoints in scope: `GET /ingest-time`, `GET /routes`, `GET /stops/near`. No `/docs` endpoint, no other GTFS tables exposed.
- Route with stdlib `net/http.ServeMux` (Go 1.22+ patterns) — no third-party router dependency.
- Local dev commands (`make ingest-dev`, `make ingest-prod`) must keep working exactly as before — the non-Lambda branch of `main()` is untouched.
- DB errors are logged server-side and answered with a generic `{"error": "internal error"}` body — never the raw error string.

---

## Task 1: `internal/geo` — Haversine distance helper

**Files:**
- Create: `internal/geo/geo.go`
- Test: `internal/geo/geo_test.go`

**Interfaces:**
- Produces: `func Haversine(lat1, lon1, lat2, lon2 float64) float64` — great-circle distance in meters. Used by Task 3's `/stops/near` handler.

- [ ] **Step 1: Write the failing tests**

```go
package geo

import (
	"math"
	"testing"
)

func TestHaversine_SamePoint(t *testing.T) {
	d := Haversine(40.7580, -73.9855, 40.7580, -73.9855)
	if d != 0 {
		t.Errorf("expected 0 distance for identical points, got %f", d)
	}
}

func TestHaversine_OneDegreeAtEquator(t *testing.T) {
	d := Haversine(0, 0, 0, 1)
	want := 111195.0
	if math.Abs(d-want) > 1000 {
		t.Errorf("expected ~%.0fm, got %.0fm", want, d)
	}
}

func TestHaversine_KnownNYCDistance(t *testing.T) {
	// Times Square (40.7580, -73.9855) to Grand Central (40.7527, -73.9772),
	// roughly 900m apart.
	d := Haversine(40.7580, -73.9855, 40.7527, -73.9772)
	if d < 700 || d > 1200 {
		t.Errorf("expected roughly 700-1200m between Times Square and Grand Central, got %.0fm", d)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/geo/... -v`
Expected: FAIL — `undefined: Haversine`

- [ ] **Step 3: Write the implementation**

```go
package geo

import "math"

const earthRadiusMeters = 6371000

// Haversine returns the great-circle distance between two lat/lon points, in meters.
func Haversine(lat1, lon1, lat2, lon2 float64) float64 {
	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusMeters * c
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/geo/... -v`
Expected: PASS (all 3 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/geo/geo.go internal/geo/geo_test.go
git commit -m "feat: add Haversine great-circle distance helper"
```

---

## Task 2: `internal/db` — read queries for last fetch time, routes, and stops

**Files:**
- Modify: `internal/db/db.go`
- Test: `internal/db/db_test.go`

**Interfaces:**
- Consumes: `gtfs.Route`, `gtfs.Stop` (existing, from `internal/gtfs`); existing `SetLastFetchTime`, `InsertRoutes`, `InsertStops`, `TruncateAll`.
- Produces:
  - `func GetLastFetchTime(ctx context.Context, conn db) (time.Time, error)`
  - `func GetRoutes(ctx context.Context, conn db) ([]gtfs.Route, error)`
  - `func GetAllStops(ctx context.Context, conn db) ([]gtfs.Stop, error)`

  All three take the package's existing (unexported) `db` interface, which Task 3's `internal/api` package will call indirectly by passing a `*pgx.Conn` (these functions are called with `*pgx.Conn` at the call site, same as every other function in this file).

**Prerequisite:** Local Postgres must be running and migrated: `make up` (from repo root) starts it and auto-runs migrations, including `ingest_state` from the earlier session.

- [ ] **Step 1: Write the failing tests**

```go
package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/mahirpatel04/transit-route-optimizer/internal/gtfs"
)

func testConn(t *testing.T) *pgx.Conn {
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
	t.Cleanup(func() { conn.Close(ctx) })

	if err := TruncateAll(ctx, conn); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return conn
}

func TestGetLastFetchTime(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()

	want := time.Now().UTC().Truncate(time.Second)
	if err := SetLastFetchTime(ctx, conn, want); err != nil {
		t.Fatalf("SetLastFetchTime: %v", err)
	}

	got, err := GetLastFetchTime(ctx, conn)
	if err != nil {
		t.Fatalf("GetLastFetchTime: %v", err)
	}
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestGetRoutes(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()

	want := []gtfs.Route{
		{RouteId: "A", RouteName: "8th Avenue Express", RouteType: 1},
		{RouteId: "B", RouteName: "6th Avenue Express", RouteType: 1},
	}
	if _, err := InsertRoutes(ctx, conn, want); err != nil {
		t.Fatalf("InsertRoutes: %v", err)
	}

	got, err := GetRoutes(ctx, conn)
	if err != nil {
		t.Fatalf("GetRoutes: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d routes, want %d", len(got), len(want))
	}
}

func TestGetAllStops(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()

	want := []gtfs.Stop{
		{StopId: "127", StopName: "Times Sq-42 St", Lat: 40.7549, Lon: -73.9871},
		{StopId: "635", StopName: "Grand Central-42 St", Lat: 40.7527, Lon: -73.9772},
	}
	if _, err := InsertStops(ctx, conn, want); err != nil {
		t.Fatalf("InsertStops: %v", err)
	}

	got, err := GetAllStops(ctx, conn)
	if err != nil {
		t.Fatalf("GetAllStops: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d stops, want %d", len(got), len(want))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `make up && go test ./internal/db/... -v`
Expected: FAIL — `undefined: GetLastFetchTime` (and similar)

- [ ] **Step 3: Extend the `db` interface and add the three functions**

In `internal/db/db.go`, extend the interface (both `*pgx.Conn` and `pgx.Tx` already satisfy `Query`/`QueryRow`, so this is a non-breaking addition):

```go
type db interface {
	CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
```

Add the three functions (place near `SetLastFetchTime`):

```go
func GetLastFetchTime(ctx context.Context, conn db) (time.Time, error) {
	var t time.Time
	err := conn.QueryRow(ctx, "SELECT last_fetch_time FROM ingest_state WHERE id = 1").Scan(&t)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get last fetch time: %w", err)
	}
	return t, nil
}

func GetRoutes(ctx context.Context, conn db) ([]gtfs.Route, error) {
	rows, err := conn.Query(ctx, "SELECT route_id, route_name, route_type FROM routes")
	if err != nil {
		return nil, fmt.Errorf("failed to query routes: %w", err)
	}
	defer rows.Close()

	var routes []gtfs.Route
	for rows.Next() {
		var r gtfs.Route
		if err := rows.Scan(&r.RouteId, &r.RouteName, &r.RouteType); err != nil {
			return nil, fmt.Errorf("failed to scan route: %w", err)
		}
		routes = append(routes, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed reading routes: %w", err)
	}
	return routes, nil
}

func GetAllStops(ctx context.Context, conn db) ([]gtfs.Stop, error) {
	rows, err := conn.Query(ctx, "SELECT stop_id, stop_name, lat, lon, parent_station FROM stops")
	if err != nil {
		return nil, fmt.Errorf("failed to query stops: %w", err)
	}
	defer rows.Close()

	var stops []gtfs.Stop
	for rows.Next() {
		var s gtfs.Stop
		var parentStation *string
		if err := rows.Scan(&s.StopId, &s.StopName, &s.Lat, &s.Lon, &parentStation); err != nil {
			return nil, fmt.Errorf("failed to scan stop: %w", err)
		}
		if parentStation != nil {
			s.ParentStation = *parentStation
		}
		stops = append(stops, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed reading stops: %w", err)
	}
	return stops, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/db/... -v`
Expected: PASS (all 3 new tests, plus no regressions in any existing behavior)

- [ ] **Step 5: Commit**

```bash
git add internal/db/db.go internal/db/db_test.go
git commit -m "feat: add GetLastFetchTime, GetRoutes, GetAllStops read queries"
```

---

## Task 3: `internal/api` — HTTP handlers and mux

**Files:**
- Create: `internal/api/api.go` (mux + JSON response helpers)
- Create: `internal/api/ingest_time.go`
- Create: `internal/api/routes.go`
- Create: `internal/api/stops_near.go`
- Test: `internal/api/api_test.go`

**Interfaces:**
- Consumes: `db.GetLastFetchTime`, `db.GetRoutes`, `db.GetAllStops` (Task 2); `geo.Haversine` (Task 1).
- Produces: `func NewMux(conn *pgx.Conn) *http.ServeMux` — used by Task 5's Lambda dispatch handler.

**Prerequisite:** Local Postgres running (`make up`), same as Task 2.

- [ ] **Step 1: Write the failing tests**

```go
// internal/api/api_test.go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/api/... -v`
Expected: FAIL — package `api` doesn't exist yet / `undefined: NewMux`

- [ ] **Step 3: Write `internal/api/api.go`**

```go
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
```

- [ ] **Step 4: Write `internal/api/ingest_time.go`**

```go
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
```

- [ ] **Step 5: Write `internal/api/routes.go`**

```go
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
```

- [ ] **Step 6: Write `internal/api/stops_near.go`**

```go
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
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `go test ./internal/api/... -v`
Expected: PASS (all 5 tests)

- [ ] **Step 8: Commit**

```bash
git add internal/api/
git commit -m "feat: add internal/api package with 3 read-only HTTP handlers"
```

---

## Task 4: Cron-vs-HTTP event detection

**Files:**
- Modify: `cmd/ingest/main.go`
- Test: `cmd/ingest/main_test.go`

**Interfaces:**
- Produces: `func isCronEvent(raw json.RawMessage) bool` — used by Task 5's Lambda handler to decide which path to take.

- [ ] **Step 1: Write the failing test**

```go
// cmd/ingest/main_test.go
package main

import "testing"

func TestIsCronEvent(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{
			name: "eventbridge scheduled event",
			raw:  `{"source":"aws.events","detail-type":"Scheduled Event","detail":{}}`,
			want: true,
		},
		{
			name: "function url http request",
			raw:  `{"version":"2.0","routeKey":"$default","rawPath":"/routes","requestContext":{"http":{"method":"GET"}}}`,
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isCronEvent([]byte(tc.raw))
			if got != tc.want {
				t.Errorf("isCronEvent(%s) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ingest/... -v -run TestIsCronEvent`
Expected: FAIL — `undefined: isCronEvent`

- [ ] **Step 3: Add the function to `cmd/ingest/main.go`**

Add near the top of the file, after imports (add `"encoding/json"` to the import block):

```go
func isCronEvent(raw json.RawMessage) bool {
	var probe struct {
		Source string `json:"source"`
	}
	_ = json.Unmarshal(raw, &probe)
	return probe.Source == "aws.events"
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/ingest/... -v -run TestIsCronEvent`
Expected: PASS (both cases)

- [ ] **Step 5: Commit**

```bash
git add cmd/ingest/main.go cmd/ingest/main_test.go
git commit -m "feat: add isCronEvent to distinguish EventBridge from HTTP events"
```

---

## Task 5: Wire the dual-purpose Lambda handler

**Files:**
- Modify: `cmd/ingest/main.go`
- Modify: `go.mod`, `go.sum` (new dependency)

**Interfaces:**
- Consumes: `isCronEvent` (Task 4), `api.NewMux` (Task 3), existing `run(ctx) error`, existing `config.Load()`.
- Produces: the Lambda's actual entrypoint behavior — no new exported interface, this is the integration point.

No automated test for this task: it requires a real Lambda runtime or a full Function URL request/response round trip to exercise meaningfully, and `run(ctx)`/DB connection setup are already covered by existing/earlier tests. Verified by build success here; end-to-end behavior is verified manually in Task 7 after deploy.

- [ ] **Step 1: Add the new dependency**

Run: `go get github.com/awslabs/aws-lambda-go-api-proxy`

- [ ] **Step 2: Replace `lambda.Start(run)` and add the `handler` function**

In `cmd/ingest/main.go`, update imports to add:

```go
"github.com/aws/aws-lambda-go/events"
"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
"github.com/mahirpatel04/transit-route-optimizer/internal/api"
```

Change `main()`:

```go
func main() {
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		lambda.Start(handler)
		return
	}

	if err := run(context.Background()); err != nil {
		log.Fatalf("ingest failed: %v", err)
	}
}
```

Add the `handler` function (place above or below `run`):

```go
func handler(ctx context.Context, raw json.RawMessage) (any, error) {
	if isCronEvent(raw) {
		return nil, run(ctx)
	}

	var req events.APIGatewayV2HTTPRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("failed to parse HTTP request event: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("config error: %w", err)
	}

	conn, err := pgx.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}
	defer conn.Close(ctx)

	return httpadapter.NewV2(api.NewMux(conn)).ProxyWithContext(ctx, req)
}
```

- [ ] **Step 3: Verify the whole module builds**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 4: Run the full test suite to confirm no regressions**

Run: `go test ./... -v`
Expected: PASS (or SKIP for DB-dependent tests if `make up` isn't running — that's fine, not a regression)

- [ ] **Step 5: Commit**

```bash
git add cmd/ingest/main.go go.mod go.sum
git commit -m "feat: dispatch Lambda invocations between cron ingest and HTTP API"
```

---

## Task 6: Infra — public Function URL on the existing Lambda

**Files:**
- Modify: `infra/infra.go`

**Interfaces:**
- Consumes: existing `ingestFunction` (`awslambda.DockerImageFunction`), existing `awscdk.NewCfnOutput` pattern (see `AuroraEndpoint` output).
- Produces: a `CfnOutput` named `ApiUrl` printed after every deploy.

- [ ] **Step 1: Add `ReservedConcurrentExecutions` to the existing function props**

In `infra/infra.go`, inside the existing `awslambda.DockerImageFunctionProps{...}` literal (the one building `ingestFunction`), add:

```go
ReservedConcurrentExecutions: jsii.Number(5),
```

- [ ] **Step 2: Add the Function URL and its output**

Immediately after the `ingestFunction := awslambda.NewDockerImageFunction(...)` block, before the `schedule := ...` block, add:

```go
fnUrl := ingestFunction.AddFunctionUrl(&awslambda.FunctionUrlOptions{
	AuthType: awslambda.FunctionUrlAuthType_NONE,
})
```

Near the existing `AuroraEndpoint` output, add a second output:

```go
awscdk.NewCfnOutput(stack, jsii.String("ApiUrl"), &awscdk.CfnOutputProps{
	Value: fnUrl.Url(),
})
```

- [ ] **Step 3: Verify the CDK app builds and synthesizes**

Run (from `infra/`):
```bash
go build ./...
export DB_MASTER_PASSWORD=placeholder
cdk synth > /dev/null
```
Expected: both succeed with no errors (synth doesn't need a real password, just a non-empty one — the check in `infra.go` only verifies the env var is set).

- [ ] **Step 4: Commit**

```bash
git add infra/infra.go
git commit -m "feat: add public Function URL and concurrency cap to ingest Lambda"
```

---

## Task 7: Deploy and manually verify (not automated — requires prod access)

This task is infrastructure-affecting and should be run by a human, or by an agent only with the user's explicit go-ahead at execution time — do not run `cdk deploy` unattended.

- [ ] **Step 1: Deploy**

From `infra/`:
```bash
export DB_MASTER_PASSWORD=$(grep POSTGRES_PASSWORD ../.env.prod | cut -d= -f2)
export DEV_IP=$(curl -s https://checkip.amazonaws.com)
cdk deploy
```

- [ ] **Step 2: Note the `ApiUrl` output**

The deploy output prints `InfraStack.ApiUrl = https://<id>.lambda-url.us-east-1.on.aws/`.

- [ ] **Step 3: Verify all 3 endpoints manually**

```bash
curl -s "$API_URL/ingest-time"
curl -s "$API_URL/routes" | head -c 300
curl -s "$API_URL/stops/near?lat=40.7580&lon=-73.9855&limit=3"
curl -s "$API_URL/stops/near"   # expect a 400 with {"error": "invalid or missing lat/lon"}
```

- [ ] **Step 4: Verify the cron path still works**

Manually invoke the Lambda with the AWS CLI or console **Test** button using an EventBridge-shaped test event (or wait for the next Monday 06:00 UTC run), then confirm `ingest-time` reflects a fresh timestamp via `curl "$API_URL/ingest-time"`.
