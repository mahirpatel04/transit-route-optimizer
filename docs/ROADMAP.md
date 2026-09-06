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
- [x] Parse + insert the remaining GTFS files, in FK order:
  - [x] `routes.txt` → `routes` table
  - [x] `calendar.txt` → `calendar` table
  - [x] `calendar_dates.txt` → `calendar_dates` table
  - [x] `trips.txt` → `trips` table
  - [x] `stop_times.txt` → `stop_times` table
- [x] Harden all parsers to read CSV columns by **header name** via a shared `header` map (`internal/gtfs/parse.go`'s `newHeader`/`h.get`), not fixed index
- [x] Replace `log.Fatalf` inside `internal/gtfs/fetch.go` with a returned `error` — a library function should never kill the process; this becomes a hard requirement once the code runs inside Lambda (Phase 3), where a fatal exit mid-invocation is harder to diagnose than a returned error
- [x] Re-ingestion strategy: `cmd/ingest/main.go` truncates all 6 tables (`db.TruncateAll`, `RESTART IDENTITY CASCADE`) and re-inserts within a single `pgx.Tx`, committed only if every insert succeeds — the ingest job can now be run repeatedly against the same database without the old `duplicate key value violates unique constraint` error

Phase 1 is fully closed out.

## Phase 2 — Migrate to Aurora Serverless

- [x] Provision an Aurora Serverless v2 (Postgres-compatible) cluster — `transit-route-optimizer` cluster in `us-east-2`, min 0 / max 1 ACU, publicly accessible, inbound Postgres (5432) restricted to a single IP via the VPC's `default` security group
- [x] Point `DATABASE_URL` at Aurora — kept local Docker Postgres for day-to-day dev rather than replacing it; added `.env.prod` (gitignored) holding the Aurora connection string alongside the existing `.env` for local dev, and added `-dev`/`-prod` variants of the relevant `make` targets (`migrate-dev`/`migrate-prod`, `ingest-dev`/`ingest-prod`) so either environment can be targeted explicitly
- [x] Run existing `goose` migrations against Aurora — `make migrate-prod` applied `20260829233211_create_tables.sql` successfully; schema now matches local
- [x] Run `make ingest-prod` to confirm the Go ingest job (fetch → parse → truncate/insert) works end-to-end against Aurora — confirmed, all 6 tables populated (verified via Postico)
- [ ] Decide how the ingest job reaches Aurora once it moves into Lambda (Phase 3): VPC networking (Lambda-in-VPC) vs. Aurora Data API vs. RDS Proxy — today's IP-allowlisted public access only works for a human's laptop, not a Lambda function

**Open decisions:**

- Infra-as-code tool for provisioning (Terraform vs. AWS CDK vs. SAM/CloudFormation) — the cluster above was created by hand via the RDS console, which is fine for one cluster but won't scale to Phase 3's Lambda + EventBridge + IAM resources; still worth picking one tool before Phase 3
- Secrets management for `DATABASE_URL`/credentials — currently a local gitignored `.env.prod` file with a plaintext password; fine for solo local use, but Lambda (Phase 3) will need Secrets Manager or SSM Parameter Store since there's no laptop filesystem to read a `.env` from
- Networking model for Lambda → Aurora (VPC + security groups vs. Data API)

## Phase 3 — Move ingestion to Lambda

- [ ] Adapt `cmd/ingest` to run as a Lambda handler (via `github.com/aws-lambda-go/lambda`) instead of a plain `main()` — same fetch → parse → insert logic, different entrypoint/trigger
- [ ] Package and deploy (depends on the IaC tool chosen in Phase 2)
- [ ] Schedule periodic re-ingestion via EventBridge (cron) — GTFS static feeds update on the agency's release cadence, not continuously
- [ ] Wire Lambda logs/errors to CloudWatch; decide alerting on ingestion failure

**Open decisions:**

- Deployment packaging (Lambda container image vs. zip + custom runtime) — container image is usually simpler for Go-with-dependencies
- Whether one Lambda ingests all GTFS files per run, or one Lambda per file behind a Step Functions workflow (Phase 1's file list — routes/calendar/trips/stop_times — is now all live, so this can be decided for real)

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

Phase 2 is functionally done (Aurora provisioned, migrated, and ingested into successfully). Settle Phase 2's remaining open decisions (IaC tool, secrets management, networking model) before starting Phase 3's Lambda work — those decisions shape how the Lambda handler, EventBridge schedule, and credentials are wired up.
