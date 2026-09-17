# Public read-only API on the existing ingest Lambda

**Status:** Approved design, not yet implemented.
**Date:** 2026-09-17

## Goal

Expose a small public, read-only HTTP API so external users/products can query
the transit data this project already ingests, without waiting on Phase 4
(time-dependent A* routing) to land first. Reuses the existing ingest Lambda
rather than standing up a second function.

## Scope

Three endpoints:

- `GET /ingest-time` — last successful GTFS ingest time
- `GET /routes` — all subway routes
- `GET /stops/near?lat=&lon=&limit=` — nearest stops to a coordinate, with distance

No `/docs` endpoint. No authentication (public, unauthenticated). No rate
limiting beyond a Lambda concurrency cap (see Infra). Not in scope: exposing
`stop_times`/`trips`/`calendar` data, or any routing/pathfinding endpoint
(that's Phase 4/5, later).

## Architecture: one Lambda, two trigger types

`cmd/ingest/main.go` remains the single Lambda entrypoint and the single
compiled binary/image (per explicit decision: one shared Lambda, not two).
Local dev is unaffected — `make ingest-dev`/`make ingest-prod` still call
`run(ctx)` directly, unchanged.

Inside the Lambda branch, `lambda.Start(run)` is replaced with a generic
handler that inspects the raw incoming event and dispatches:

```go
func handler(ctx context.Context, raw json.RawMessage) (any, error) {
    var probe struct {
        Source string `json:"source"` // "aws.events" for EventBridge cron
    }
    json.Unmarshal(raw, &probe)

    if probe.Source == "aws.events" {
        return nil, run(ctx) // existing ingest path, unchanged
    }
    return httpadapter.NewV2(apiMux(conn)).ProxyWithContext(ctx, raw) // Function URL request
}
```

- EventBridge scheduled events carry `"source": "aws.events"` — routes to the
  existing `run(ctx)` ingest path, untouched.
- Anything else (a Lambda Function URL request, which uses the API Gateway
  v2 HTTP payload format) is adapted into a real `http.Request` via
  `github.com/awslabs/aws-lambda-go-api-proxy/httpadapter` and routed through
  a standard `net/http.ServeMux`.
- The DB connection (`*pgx.Conn`, via `config.Load()` + `pgx.Connect`) is
  opened fresh per invocation on the HTTP path, same as the existing ingest
  path already does — simpler than reusing a connection across warm/cold
  Lambda cycles, and avoids needing to health-check a possibly-stale
  connection after a freeze.

`net/http.ServeMux`'s Go 1.22+ method+path-variable patterns
(`mux.HandleFunc("GET /stops/near", ...)`) are used directly — no external
router dependency needed for 3 routes.

## New packages

### `internal/geo` (new)

```go
func Haversine(lat1, lon1, lat2, lon2 float64) float64 // returns meters
```

Small, dependency-free great-circle distance helper. Deliberately factored
out here (not inlined into the stops handler) because Phase 4's A* heuristic
will need the identical calculation later.

### `internal/db` (additions)

- `GetLastFetchTime(ctx, conn) (time.Time, error)` — reads the single row
  from `ingest_state` (counterpart to the existing `SetLastFetchTime`).
- `GetRoutes(ctx, conn) ([]gtfs.Route, error)` — `SELECT route_id, route_name,
  route_type FROM routes`.
- `GetAllStops(ctx, conn) ([]gtfs.Stop, error)` — `SELECT stop_id, stop_name,
  lat, lon, parent_station FROM stops`. Full-table fetch is fine — NYC subway
  `stops` is ~1,500 rows including platforms; distance computation and
  sorting happens in Go, not SQL, avoiding any need for PostGIS/geo SQL.

### `internal/api` (new)

- `NewMux(conn *pgx.Conn) *http.ServeMux` — builds and returns the 3-route
  mux, each handler closing over `conn`.
- `writeJSON(w http.ResponseWriter, status int, v any)` — single response
  helper used by all 3 handlers.
- Handlers:
  - `handleIngestTime` — calls `db.GetLastFetchTime`, returns
    `{"last_fetch_time": "<RFC3339>"}`.
  - `handleRoutes` — calls `db.GetRoutes`, returns a JSON array of
    `{"route_id", "route_name", "route_type"}`.
  - `handleStopsNear` — parses/validates `lat`, `lon` (required, floats;
    missing or unparsable → `400`), `limit` (optional int, default 10, capped
    at 50 — invalid values silently fall back to the default rather than
    erroring). Calls `db.GetAllStops`, computes `geo.Haversine` from the
    query point to each stop, sorts ascending, returns the top `limit` as
    `{"stop_id", "stop_name", "lat", "lon", "distance_meters"}`.

## Error handling

- Any DB error or unexpected failure: logged server-side (`log.Printf`,
  visible in CloudWatch Logs) and answered with a generic body — never the
  raw Go error string, to avoid leaking schema/connection details publicly.
  - `400 {"error": "invalid or missing lat/lon"}` — validation failures.
  - `500 {"error": "internal error"}` — DB/unexpected failures; real error
    only in logs.
- The cron ingest path (`run(ctx)`) is untouched: its errors still propagate
  as a Lambda invocation failure (CloudWatch), since there's no HTTP caller
  to respond to on that path.

## Infra changes (`infra/infra.go`)

- Add a **Function URL** to the existing `IngestFunction`:
  `AddFunctionUrl(&awslambda.FunctionUrlOptions{AuthType:
  awslambda.FunctionUrlAuthType_NONE})`. This is the only new public-facing
  resource — no API Gateway.
- Add `ReservedConcurrentExecutions: jsii.Number(5)` to the
  `DockerImageFunctionProps` — caps concurrent executions across *both* the
  cron and API paths, since they share one function, guarding against a
  public traffic spike hammering Aurora.
- A `CfnOutput` printing the Function URL after deploy (same pattern as the
  existing `AuroraEndpoint` output).
- No VPC/security-group/Aurora changes — the Lambda already has DB access
  via its existing VPC attachment; this only adds a second trigger.

## Explicitly out of scope

- `/docs` endpoint or any generated API documentation (declined — revisit if
  this gets real external users).
- Authentication / API keys.
- Rate limiting beyond the Lambda concurrency cap (no API Gateway, no
  per-client throttling).
- Any routing/pathfinding endpoint (depends on Phase 4, not yet built).
- Exposing `stop_times`, `trips`, `calendar`, or `calendar_dates` data.

## Testing

- Unit tests for `internal/geo.Haversine` — known-distance fixtures (e.g. two
  real NYC coordinates with a known real-world distance, asserting within a
  reasonable tolerance).
- Unit tests for `db.GetRoutes`, `db.GetAllStops`, `db.GetLastFetchTime`
  against local Docker Postgres, following the existing `internal/db` test
  patterns.
- Unit tests for the 3 `internal/api` handlers via `net/http/httptest`,
  covering: success shape/status for each endpoint, and the `400` path for
  missing/invalid `lat`/`lon` on `/stops/near`.
- Manual end-to-end verification after deploy: `curl` all 3 endpoints against
  the real Function URL, plus one EventBridge-triggered (or manually
  invoked) ingest run to confirm the cron path still works unchanged.
