// Package routing implements a time-dependent A* search over the GTFS
// schema to find the fastest path between subway stops.
//
// Tests require local Postgres with real ingested GTFS data (`make up &&
// make ingest-dev`) and skip silently — not fail — when DATABASE_URL is
// unset; use `-v` to see "SKIP" rather than a misleading passing `ok`.
package routing
