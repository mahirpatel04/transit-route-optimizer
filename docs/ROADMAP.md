# Transit Route Optimizer — Execution Roadmap

**Goal:** Ingest NYC transit GTFS data, land it in a serverless Postgres database (Aurora Serverless), automate re-ingestion on AWS Lambda, and build a time-dependent A* algorithm that finds optimal routes using real schedule data.

**How to read this doc:** Phases are roughly sequential. Checked items are already done in this repo. Each open phase lists concrete next steps where the approach is already decided, and **Open decisions** where it isn't yet — those need a design pass before they can be broken into a real implementation plan (see `superpowers:writing-plans` once a phase's decisions are settled).

---

## Phase 0 — Project foundations

- [x] Go module + repo structure (`cmd/`, `internal/`)
- [x] Local Postgres via `docker-compose.yml`
- [x] Schema migrations tool: `goose`, run via `dockerfile.goose` (`migrations/`)
- [x] Initial schema v1: `stops`, `routes`, `calendar`, `calendar_dates`, `trips`, `stop_times` (`migrations/20260829233211_create_tables.sql`)
- [x] `internal/config`: loads `DATABASE_URL` from env / `.env` via `godotenv`

## Phase 1 — GTFS ingestion (local)

- [x] `internal/gtfs.FetchData`: download GTFS zip over HTTP
- [x] `internal/gtfs.OpenZip` / `ReadFileFromZip`: extract a file from the zip in memory
- [x] `internal/gtfs.ParseStopFile`: parse `stops.txt` into `[]Stop`
- [x] `internal/db.InsertStops`: bulk-load stops into Postgres via `pgx.CopyFrom`
- [x] `cmd/ingest/main.go`: wire fetch → unzip → parse → insert for stops
- [ ] Parse + insert the remaining GTFS files, in FK order:
  - [ ] `routes.txt` → `routes` table
  - [ ] `calendar.txt` → `calendar` table
  - [ ] `calendar_dates.txt` → `calendar_dates` table
  - [ ] `trips.txt` → `trips` table
  - [ ] `stop_times.txt` → `stop_times` table (largest file; do this last)
- [ ] Harden `ParseStopFile` (and the new parsers) to read CSV columns by **header name**, not fixed index — GTFS feeds vary column order/optional columns, most likely to bite on `stop_times.txt`
- [ ] Replace `log.Fatalf` inside `internal/gtfs/fetch.go` with a returned `error` — a library function should never kill the process; this becomes a hard requirement once the code runs inside Lambda (Phase 3), where a fatal exit mid-invocation is harder to diagnose than a returned error
- [ ] Decide re-ingestion strategy: full truncate-and-reload per run, or upsert/diff — GTFS feeds republish periodically and stop/trip IDs can be reused across feed versions

**Open decisions:** none blocking — this phase is a straightforward extension of the existing pattern.

## Phase 2 — Migrate to Aurora Serverless

- [ ] Provision an Aurora Serverless v2 (Postgres-compatible) cluster
- [ ] Point `DATABASE_URL` at Aurora instead of local `docker-compose` Postgres; confirm `pgx` connects (Aurora Serverless v2 speaks the Postgres wire protocol, so this should be close to a config-only change)
- [ ] Run existing `goose` migrations against Aurora
- [ ] Decide how the ingest job reaches Aurora across environments (VPC networking: Lambda-in-VPC vs. Aurora Data API vs. RDS Proxy)

**Open decisions:**

- Infra-as-code tool for provisioning (Terraform vs. AWS CDK vs. SAM/CloudFormation) — affects how Phase 3's Lambda is defined too, worth picking once for both
- Secrets management for `DATABASE_URL`/credentials (Secrets Manager vs. SSM Parameter Store)
- Networking model for Lambda → Aurora (VPC + security groups vs. Data API)

## Phase 3 — Move ingestion to Lambda

- [ ] Adapt `cmd/ingest` to run as a Lambda handler (via `github.com/aws-lambda-go/lambda`) instead of a plain `main()` — same fetch → parse → insert logic, different entrypoint/trigger
- [ ] Package and deploy (depends on the IaC tool chosen in Phase 2)
- [ ] Schedule periodic re-ingestion via EventBridge (cron) — GTFS static feeds update on the agency's release cadence, not continuously
- [ ] Wire Lambda logs/errors to CloudWatch; decide alerting on ingestion failure

**Open decisions:**

- Deployment packaging (Lambda container image vs. zip + custom runtime) — container image is usually simpler for Go-with-dependencies
- Whether one Lambda ingests all GTFS files per run, or one Lambda per file behind a Step Functions workflow (matters once Phase 1's file list — routes/calendar/trips/stop_times — is all live)

## Phase 4 — Time-dependent A* routing

- [ ] Design the routing graph model: nodes = stops, edges = scheduled trips between consecutive `stop_times` rows, edge weight = wait time + travel time as a function of departure time (this is what makes it "time-dependent" rather than a static shortest-path graph)
- [ ] Decide how to query "next departure from stop X after time T" efficiently from `stop_times` (likely an index on `(stop_id, departure_time)`, already partially covered by `idx_stop_times_stop_id`)
- [ ] Implement the time-dependent A* core: priority queue keyed by arrival time, admissible heuristic (e.g. great-circle distance / max vehicle speed) using `stops.lat`/`lon`
- [ ] Handle `calendar` / `calendar_dates` service-day filtering (a trip only "exists" on days its service_id is active)
- [ ] Validate against known real routes (e.g. compare computed NYC subway A-to-B trips against Google Maps transit directions for sanity)

**Open decisions:**

- Where the algorithm runs (batch precomputation vs. on-demand query — Lambda cold-start cost matters if this becomes a live query service)
- Whether results are exposed via an API, CLI, or stay a library used by tests/benchmarks (not yet requested, but likely the natural next phase after A* itself works)

---

## Suggested immediate next step

Finish Phase 1 (the remaining GTFS parsers) before touching AWS — it's the same pattern you already know, it's what Phase 4's algorithm will query against, and it de-risks the header-based-parsing fix while there's only one file's worth of pain to feel instead of five.
