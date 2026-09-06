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

**Decisions made:**

- Infra-as-code tool: **AWS CDK (Go)** — chosen over Terraform since Terraform proficiency needs modules/remote state/multi-env to be resume-worthy, which is more setup than this project justifies; CDK in Go doubles as more Go practice instead. The Aurora cluster above was created by hand via the RDS console and won't be retroactively imported into CDK — CDK starts with Phase 3's new resources (Lambda, EventBridge, IAM).

- Secrets management: **AWS Secrets Manager, single secret** — one secret object holding all key-value pairs (db host, user, password, dbname, etc.) rather than one secret per value, since Secrets Manager bills per secret (~$0.40/month) regardless of how many keys it holds. Lambda (Phase 3) will fetch and assemble `DATABASE_URL` from this secret at cold start, replacing the local `.env.prod` file read.
- Networking model: **Lambda-in-VPC**, keeping the existing `pgx`/`CopyFrom` wiring unchanged — no Data API rewrite needed. Watch for NAT gateway cost (~$32/month) if the Lambda also needs outbound internet access to fetch the GTFS zip from S3/its public URL.

**Open decisions:** none remaining — Phase 2 is fully closed out.

## Phase 3 — Move ingestion to Lambda

**Goal:** run the existing GTFS ingest job on a recurring schedule in AWS, without a human manually running `make ingest-prod`.

**Decisions made:**

- **Packaging:** container image, pushed to ECR, referenced by the Lambda function definition. Chosen over zip + custom runtime for simplicity with a Go binary (reuses the `dockerfile.goose`-style pattern already in this repo).
- **Lambda granularity:** one Lambda function ingests all 6 GTFS files per invocation, matching today's `cmd/ingest/main.go` behavior exactly. No Step Functions / per-file Lambdas — the whole ingest run takes seconds and doesn't need parallelization or per-file failure isolation.
- **Entrypoint:** no new `cmd/` directory. `cmd/ingest/main.go` gets a single new entrypoint that behaves as both a local CLI and a Lambda handler (see below), since Go disallows two `func main()` in one package and a second directory was ruled out to avoid unnecessary structure.
- **Schedule:** EventBridge cron rule, weekly. Matches typical GTFS static feed republish cadence; avoids invoking daily for a feed that rarely changes.
- **Alerting:** none beyond default Lambda → CloudWatch Logs. No CloudWatch Alarms/SNS — a personal project doesn't need paged alerting; logs are enough if something needs debugging.
- **Secrets:** the single AWS Secrets Manager secret from Phase 2 holds all DB connection fields (host, user, password, dbname, port) as one JSON object. Lambda fetches and assembles `DATABASE_URL` from this secret at cold start, replacing the local `.env.prod` file read.
- **Networking:** Lambda attaches to the same VPC as the Aurora cluster ("Lambda-in-VPC"), per the Phase 2 networking decision — keeps the existing `pgx`/`CopyFrom` Postgres wiring completely unchanged, no Aurora Data API rewrite. Tradeoff: the Lambda also needs outbound internet access to fetch the GTFS zip from its public URL, so a NAT Gateway is required while VPC-attached — that has its own ~$32/month cost, called out explicitly so it isn't a surprise on the AWS bill.
- **IaC tool:** AWS CDK, in Go (Phase 2 decision). The existing Aurora cluster (created by hand via the RDS console) is *not* retroactively imported into CDK; CDK ownership starts with Phase 3's new resources (Lambda, EventBridge rule, IAM role, Secrets Manager secret).

**Design:**

1. **`cmd/ingest/main.go` — dual-mode entrypoint.** Today's `main()` body (connect → fetch/parse all 6 files → begin tx → truncate → insert all 6 → commit) is extracted into a `run(ctx context.Context) error` function, unchanged in behavior. The new `main()` checks `os.Getenv("AWS_LAMBDA_FUNCTION_NAME")` — if set (Lambda auto-sets this on every invocation), call `lambda.Start(func(ctx) error { return run(ctx) })`; if not (local dev), call `run(context.Background())` and `log.Fatalf` on error, same as today. Local dev (`make ingest-dev`/`make ingest-prod`) is completely unaffected since that env var is never set outside Lambda.
2. **`internal/config.Load()` — Secrets Manager branch.** Same env var check decides the config source: unset → today's exact `godotenv.Load()` + `os.Getenv("DATABASE_URL")` path, untouched; set → fetch the secret (name passed via a Lambda environment variable, e.g. `DB_SECRET_ARN`, set by CDK) using `github.com/aws/aws-sdk-go-v2/service/secretsmanager`, and assemble the same `postgres://...` URL format from its `host`/`port`/`user`/`password`/`dbname` keys. `Config` struct and every caller of `Load()` stay unchanged.
3. **Packaging — `Dockerfile.lambda`.** New file at repo root (sibling to `dockerfile.goose`). Multi-stage build: compile the Go binary statically, copy into an AWS-provided Lambda base image for Go, set the binary as the image's handler entrypoint per AWS's container-image Lambda contract.
4. **Infrastructure — `infra/` (new top-level directory, CDK app in Go).** One stack: an ECR image asset (CDK's `DockerImageAsset`, built from `Dockerfile.lambda`, so no manual `docker push` step); the Secrets Manager secret (not auto-populated with the real password by CDK — set manually once via CLI/console after `cdk deploy`, same as the Aurora master password); the Lambda function (container image, attached to Aurora's VPC/security group, IAM role scoped to `secretsmanager:GetSecretValue` on just that one secret); an EventBridge rule (weekly cron, target = the Lambda). No API Gateway, no Step Functions.

**Testing plan:**

- Local: `run(ctx)` is already exercised via `make ingest-dev`/`make ingest-prod` — no regression expected since its logic is extracted, not rewritten.
- Lambda: after `cdk deploy`, manually invoke via `aws lambda invoke` and confirm rows land in Aurora (same row-count check used for `make ingest-prod`).
- EventBridge: confirm the rule appears in the console with the correct weekly cron expression and the Lambda as its target — a configuration review, not a live end-to-end trigger test.

**Out of scope:** CloudWatch Alarms/SNS alerting; retrofitting the hand-created Aurora cluster into CDK; any changes to `internal/gtfs` or `internal/db`.

**Open decisions:** none remaining — ready to implement.

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

Phase 2 is fully done — Aurora provisioned/migrated/ingested, and all three open decisions (CDK in Go, single Secrets Manager secret, Lambda-in-VPC) are settled. Start Phase 3: adapt `cmd/ingest` into a Lambda handler.
