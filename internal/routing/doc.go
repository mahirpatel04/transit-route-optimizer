// Package routing implements a time-dependent A* search over the existing
// GTFS schema (stops, routes, trips, stop_times, calendar, calendar_dates)
// to find the fastest path between two subway stops.
//
// Running this package's tests requires local Postgres with real ingested
// GTFS data: `make up && make ingest-dev` from the repo root. Tests skip
// silently (not fail) when DATABASE_URL is unset — `go test ./internal/routing/... -v`
// (with -v) will show "SKIP" for every test in that case; without -v it
// looks like an ordinary passing `ok` even though nothing was asserted.
package routing
