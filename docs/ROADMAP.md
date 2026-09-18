# Build History

Phase-by-phase summary of how this project was built. See [`ARCHITECTURE.md`](ARCHITECTURE.md) for how the current system fits together.

## Phase 0-1 — Foundations + GTFS ingestion (local)

- Go module structure, local Postgres (`docker-compose.yml`), `goose` migrations
- `internal/gtfs`: fetch GTFS zip, parse 6 CSVs by header name (not fixed column index — real-world files reorder/omit columns)
- `internal/db`: bulk load via `pgx.CopyFrom`, truncate-and-reload in one transaction (safe to re-run)

## Phase 2 — Aurora Serverless

- Migrated from local Postgres to Aurora Serverless v2. Originally hand-clicked through the console in `us-east-2`; fully superseded by Phase 3's CDK-managed cluster in `us-east-1`.

## Phase 3 — Move ingestion to Lambda

- Packaged as a container image (`dockerfile.lambda`) — the base image's entrypoint expects an interpreted-runtime handler name, so `ENTRYPOINT` is overridden to run the compiled Go binary directly.
- `cmd/ingest/main.go` branches on `AWS_LAMBDA_FUNCTION_NAME`: `lambda.Start(run)` vs. calling `run()` directly for local dev.
- Weekly EventBridge cron trigger.
- **Secrets:** dropped a planned Secrets Manager integration — a VPC-attached Lambda with no NAT can't reach its public endpoint, and a private interface endpoint isn't worth ~$7-8/mo for a value that never rotates. `DATABASE_URL` is a plain Lambda env var instead.
- **Region:** moved the whole stack to `us-east-1` (same region as the GTFS S3 bucket) so a free S3 gateway VPC endpoint could replace a NAT Gateway/Instance entirely.
- **IaC:** brought Aurora into CDK alongside everything else once the region move required rebuilding it anyway.

## Phase 4 — Time-dependent A* routing

- `internal/routing`: A* over stops/scheduled trips, admissible heuristic (great-circle distance / 27 m/s), `calendar`/`calendar_dates` service-day filtering, transfer edges between complexes.
- Validated against Google Maps transit directions — used to find and fix the two bugs described in Phase 5.

## Phase 5 — HTTP API, geocoding, infra evolution

- `GET /route?from=&to=&depart_at=` and `GET /ingest-time`; removed two placeholder endpoints (`/routes`, `/stops/near`) once the real UI existed.
- **Geocoding:** started with public Nominatim (rate-limited, required a NAT instance for internet access — 5 commits of AMI/networking fixes). Switched to AWS Location Service, which has its own VPC interface endpoint — the NAT instance and private subnet were deleted entirely rather than patched further.
- **Fixed a request-timeout bug:** trying 3 candidate platforms at each end meant up to 9 full A* searches per request. Reduced to 1 (later recovered via multi-target search in Phase 7, without the extra cost).
- **Fixed a transfer-matching bug, found via a Google Maps comparison:** cross-line transfers required an exact `stop_name` match, missing real transfers where MTA names the same physical complex differently per line. Fixed by matching on proximity (250m) instead.
- Added the missing address↔platform walk legs at both ends of a route, plus station names and line info on every leg.

## Phase 6 — Frontend

- React (Vite) app, deployed to GitHub Pages. `RouteFinder`: address inputs, suggestion chips for popular landmarks, colored line bullets, "spinning up the database" note for Aurora's cold-start-from-zero.
- Collapses zero-duration same-station walk legs in the display (a cross-complex transfer lands on a parent node, then hops to the boarding platform — two real edges that read as redundant if shown separately).

## Phase 7 — Multi-target search

- The default strategy always resolves to the single nearest destination platform, then finds the time-optimal path to it — never asking whether a farther-to-walk platform on a better line would be faster overall.
- `FindRouteMultiTarget`: one A* search across multiple acceptable destination platforms, using a multi-goal heuristic (min estimated time across all targets). Same DB cost as a single-target search.
- `?optimize=true` opts into 3 destination candidates instead of 1. Verified across 6 real trips: never slower than the default, up to 29% faster when the nearest platform sits on a worse line.
- Frontend: a slider toggle between "closest station" (default) and "fastest trip."

## Next up

- Make candidate counts configurable without a redeploy.
- Benchmark `/route` against the Google Maps Directions API across more trips to find the next-biggest gap (likely schedule/frequency modeling, not platform selection).
