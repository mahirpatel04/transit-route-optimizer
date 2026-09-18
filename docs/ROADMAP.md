# Transit Route Optimizer — Execution Roadmap

**Goal:** Ingest NYC transit GTFS data, land it in a serverless Postgres database (Aurora Serverless), automate re-ingestion on AWS Lambda, build a time-dependent A* algorithm that finds optimal routes using real schedule data, and serve it over an HTTP API and frontend given plain NYC addresses.

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

- [x] Provision an Aurora Serverless v2 (Postgres-compatible) cluster
- [x] Point `DATABASE_URL` at Aurora — kept local Docker Postgres for day-to-day dev rather than replacing it; `.env.prod` (gitignored) holds the Aurora connection string alongside `.env` for local dev, with `-dev`/`-prod` variants of the relevant `make` targets (`migrate-dev`/`migrate-prod`, `ingest-dev`/`ingest-prod`)
- [x] Run `goose` migrations against Aurora
- [x] Confirm the Go ingest job (fetch → parse → truncate/insert) works end-to-end against Aurora — verified via Postico/psql row counts

**Note:** this phase was originally done by hand via the RDS console with the cluster in `us-east-2`, and later fully superseded — see Phase 3, which rebuilt Aurora as a CDK-managed resource in `us-east-1` after discovering the GTFS feed's S3 bucket lives in `us-east-1` (see Phase 3's networking decision). The cluster clicked through here no longer exists; its replacement is defined in `infra/infra.go`.

**Open decisions:** none remaining — Phase 2 is fully closed out.

## Phase 3 — Move ingestion to Lambda

**Goal:** run the existing GTFS ingest job on a recurring schedule in AWS, without a human manually running `make ingest-prod`.

**Decisions made (some superseding earlier Phase 2 decisions, per what actually worked):**

- **Packaging:** container image, pushed to ECR, referenced by the Lambda function definition (`dockerfile.lambda` at repo root, sibling to `dockerfile.goose`). The base image's default entrypoint expects a "handler name" argument (built for interpreted runtimes) — our Go binary implements the full Lambda Runtime API loop itself via `aws-lambda-go`, so `dockerfile.lambda` overrides `ENTRYPOINT` to run the compiled `bootstrap` binary directly.
- **Lambda granularity:** one Lambda function ingests all 6 GTFS files per invocation, matching `cmd/ingest/main.go`'s existing behavior. No Step Functions / per-file Lambdas.
- **Entrypoint:** no new `cmd/` directory. `cmd/ingest/main.go`'s `main()` extracts its body into `run(ctx) error`, then branches on `os.Getenv("AWS_LAMBDA_FUNCTION_NAME")` — set (Lambda) → `lambda.Start(run)`; unset (local dev) → call `run` directly and `log.Fatalf` on error, same as before. `make ingest-dev`/`make ingest-prod` are unaffected.
- **Schedule:** EventBridge cron rule, weekly (`WeekDay: MON, Hour: 6, Minute: 0` UTC).
- **Alerting:** none beyond default Lambda → CloudWatch Logs.
- **Secrets — superseded:** originally planned as a single AWS Secrets Manager secret (Phase 2 decision). Dropped after deployment: a VPC-attached Lambda with no NAT can't reach Secrets Manager's public endpoint (its own separate networking problem from the S3 one below), and a Secrets Manager VPC interface endpoint costs ~$7-8/month — not worth it for a value with no rotation needs. **`DATABASE_URL` is instead passed as a plain Lambda environment variable** (AWS encrypts these at rest by default), built directly in `infra/infra.go` from the Aurora cluster's token-resolved hostname plus a `DB_MASTER_PASSWORD` value exported before `cdk deploy`/`cdk synth`. `internal/config.Load()` no longer branches on Lambda at all — it's back to the same single `godotenv`/`os.Getenv("DATABASE_URL")` path used locally, since Lambda now gets `DATABASE_URL` directly as an env var too.
- **Networking — region moved after a real deploy failure:** Lambda is VPC-attached ("Lambda-in-VPC"), same as originally decided, keeping `pgx`/`CopyFrom` unchanged. The free-NAT-avoidance trick (an S3 gateway VPC endpoint, since the GTFS zip is hosted on S3) only works when the **VPC and the S3 bucket are in the same region** — first deploy put Aurora/Lambda in `us-east-2` while the GTFS bucket (`rrgtfsfeeds`) is in `us-east-1`, so the endpoint didn't cover that traffic and every invocation timed out fetching the zip. Fix: **moved everything to `us-east-1`** (Aurora, VPC, Lambda) rather than paying for a NAT Gateway (~$32/month) or NAT Instance (~$3-4/month) — $0 extra cost once same-region. `awslambda.DockerImageFunctionProps.AllowPublicSubnet` is set to `true` since CDK's default safety check doesn't know the S3-endpoint-only traffic pattern is fine without real internet access.
- **IaC tool:** AWS CDK, in Go. **Scope changed from the original Phase 2 decision**: Aurora is no longer hand-created and excluded from CDK — since the region move required rebuilding it anyway, the Aurora cluster itself is now a CDK-managed resource (`awsrds.NewDatabaseCluster` in `infra/infra.go`), not just the Lambda/EventBridge/IAM pieces.

**Design (as actually built, in `infra/infra.go` + `dockerfile.lambda`):**

1. **`cmd/ingest/main.go` — dual-mode entrypoint**, as described above.
2. **`internal/config.Load()`** — unchanged from its original local-dev form (see Phase 2/Phase 1); no Lambda-specific branch needed now that Secrets Manager was dropped.
3. **`dockerfile.lambda`** — multi-stage build: `golang:1.27-alpine` compiles a static `bootstrap` binary (`GOOS=linux GOARCH=amd64`), copied into `public.ecr.aws/lambda/provided:al2023` with `ENTRYPOINT ["/var/task/bootstrap"]`. Named lowercase (not `Dockerfile.lambda`) to match this repo's existing `dockerfile.goose` convention — also worked around an intermittent Docker Desktop file-resolution issue specific to that exact mixed-case filename.
4. **`.dockerignore`** (repo root, new) — excludes `.git`, `infra/cdk.out`, `infra/node_modules`, `docs`. Without this, `DockerImageCode_FromImageAsset("..")`'s build context recursively copied `infra/cdk.out` into itself (the very directory CDK stages that asset into), causing an `ENAMETOOLONG` crash on `cdk bootstrap`/`cdk synth`.
5. **`infra/` (CDK app in Go)** — one stack (`infra/infra.go`):
   - Imports the account's **default VPC** in whatever region `env()` targets (`us-east-1`).
   - An **S3 gateway endpoint** on that VPC (free, same-region only — see networking decision above).
   - Two security groups: one for Aurora, one for the Lambda; the Lambda's SG is granted ingress on 5432 into Aurora's SG. An optional `DEV_IP` env var, if set at deploy time, also opens 5432 to that one IP so a human can `psql` in directly — without it, only the Lambda can reach Aurora.
   - **`awsrds.NewDatabaseCluster`**: Aurora Serverless v2 (`AuroraPostgresEngineVersion_VER_17_7`, `ServerlessV2MinCapacity: 0`, `ServerlessV2MaxCapacity: 1`), publicly accessible, credentials via `Credentials_FromPassword` using a `DB_MASTER_PASSWORD` env var (not Secrets Manager — see above), default database name `transit_optimizer`.
   - `DATABASE_URL` is built via `fmt.Sprintf` combining the plain username/password (both `url.QueryEscape`'d — a `^` in the password broke the connection string twice before this was added) with the cluster's `ClusterEndpoint().Hostname()` **CDK token** — string concatenation with a token works because CDK detects the embedded token text and resolves it into a CloudFormation `Fn::Join` automatically wherever the resulting string is used as a resource prop.
   - **`awslambda.NewDockerImageFunction`**: built from `dockerfile.lambda` via `DockerImageCode_FromImageAsset`, VPC-attached, `AllowPublicSubnet: true`, `DATABASE_URL` passed as a plain environment variable, 60s timeout, 512MB memory.
   - **EventBridge rule**: weekly cron, target = the Lambda.
   - A `CfnOutput` prints the Aurora endpoint after deploy, since it's otherwise only knowable via the console/CLI.

**Testing (as actually done):**

- Local: `run(ctx)` continues to work via `make ingest-dev`/`make ingest-prod` — confirmed no regression after the entrypoint refactor.
- Lambda: manually invoked via the console **Test** button and via `aws lambda invoke` — iterated through three real failures before success (Secrets Manager unreachable from a VPC-attached Lambda with no NAT → dropped Secrets Manager for a plain env var; then a cross-region S3 timeout → moved everything to `us-east-1`; then an `invalid userinfo` URL-parse error from the `^` in the password not being escaped in `infra.go`'s `fmt.Sprintf` → added `url.QueryEscape`). After all three fixes, a manual invoke completed successfully and Aurora showed all 6 tables populated.
- EventBridge: rule and cron expression confirmed via `cdk synth` output; not yet observed firing on its real weekly schedule (a configuration review, not a live trigger test).

**Out of scope:** CloudWatch Alarms/SNS alerting; a Secrets Manager VPC interface endpoint (rejected as not worth ~$7-8/month for a value with no rotation needs); any changes to `internal/gtfs` or `internal/db`.

**Open decisions:** none remaining — Phase 3 is functionally done. Housekeeping still worth doing: confirm the old `us-east-2` `InfraStack` and Aurora cluster are fully torn down (deletion was in progress via `aws cloudformation delete-stack` last checked), and consider whether `DEV_IP` should be re-exported on every future `cdk deploy` from a new network or handled some other way long-term.

## Phase 4 — Time-dependent A* routing

- [x] Design the routing graph model: nodes = stops, edges = scheduled trips between consecutive `stop_times` rows, edge weight = wait time + travel time as a function of departure time (this is what makes it "time-dependent" rather than a static shortest-path graph)
- [x] Decide how to query "next departure from stop X after time T" efficiently from `stop_times` — indexed on `(stop_id, departure_time)` (`migrations/20260917221938_add_stop_times_departure_index.sql`)
- [x] Implement the time-dependent A* core: priority queue keyed by arrival time, admissible heuristic (great-circle distance / max vehicle speed, 27 m/s) using `stops.lat`/`lon` (`internal/routing/routing.go`)
- [x] Handle `calendar` / `calendar_dates` service-day filtering (`internal/routing/service_days.go`)
- [x] Transfer edges: free between platforms sharing a `parent_station`, distance-priced walk between any two station complexes within 250m (`internal/routing/transfers.go`) — originally matched cross-line transfers by exact `stop_name`, which missed real transfers where MTA names the same physical complex differently per line; fixed to match by proximity instead (see Phase 5)
- [x] Validate against known real routes — compared against Google Maps transit directions; used to find and fix both the candidate-fan-out timeout and the `stop_name`-matching bug (Phase 5)

Phase 4's algorithm is done and exposed via the API (Phase 5). Open items from here are refinements, not gaps in the core search: `candidateStopCount` is currently 1 (only the single nearest platform at each end) purely to avoid the request-timeout bug documented in Phase 5 — batching/caching the per-expansion DB queries would let this go back to trying multiple candidates without timing out, which would likely close more of the remaining gap to Google Maps' routing quality than anything else on this list.

## Phase 5 — HTTP API, geocoding, and infra evolution

**Goal:** expose Phase 4's routing engine over HTTP, given two free-text addresses instead of stop IDs.

- [x] `GET /route?from=<address>&to=<address>&depart_at=<RFC3339, optional>` (`internal/api/route.go`): geocodes both addresses, resolves each to its nearest subway platform, runs `FindRoute`, and returns the trip as JSON — total time plus ordered walk/ride legs, each with station names and (for rides) the line to board
- [x] `GET /ingest-time` — last successful ingest timestamp, surfaced in the frontend header
- [x] Removed two placeholder endpoints (`GET /routes`, `GET /stops/near`) and their stub frontend panels once the real `/route` UI existed — they'd served their purpose as early scaffolding
- [x] **Geocoding, round one — Nominatim:** the public OpenStreetMap Nominatim API, bounded to a NYC viewbox, with a required `User-Agent` header, client-side rate limiting (max 1 req/sec, its usage policy), and retry-with-backoff
- [x] **Geocoding, round two — AWS Location Service:** switched from Nominatim to AWS Location Service (Places, Esri data source) for a higher rate limit suitable beyond hobby traffic. This ended up mattering more than expected — see the NAT detour below.
- [x] **The NAT detour and its removal:** Nominatim needed real internet access, so a NAT instance + private subnet were added (five commits fixing AMI architecture, network-interface detection, and iptables/nftables issues along the way). Once geocoding moved to AWS Location Service, its own VPC interface endpoint meant the Lambda needed **no internet access at all** — the NAT instance and private subnet were deleted entirely. Net effect: less infrastructure than before, not more, once the actual dependency (a geocoder needing internet) was replaced with one that didn't.
- [x] **Route search fan-out caused request timeouts:** trying the 3 nearest candidate platforms at each end meant up to 9 independent A* searches per request, each issuing many sequential DB round trips — enough to exceed the Lambda's timeout under real traffic. Reduced `candidateStopCount` to 1.
- [x] **Cross-complex transfer matching bug, found via a Google Maps comparison:** a real trip (Long Island City → W 50th St) came back at 41 minutes through 3 line changes, versus Google's ~19-29 minutes via a single direct line. Root cause: `internal/routing/transfers.go` required an exact `stop_name` match to connect two lines' stations, which silently missed real transfers where MTA names the same physical complex differently per line (e.g. "Court Sq" vs. "Court Sq-23 St", ~90m apart). Fixed by matching any two complexes within 250m instead, priced by actual walking distance rather than a flat cost — the same trip improved to 30 minutes once the search could see the transfer it was missing.
- [x] Added the missing address↔platform walk legs at both ends of a route (previously the response silently started/ended at the nearest platform with no accounting for the real walk from/to the typed address) and station names + line info on every leg.

**Open decisions:** none blocking — `candidateStopCount` staying at 1 and the remaining gap to Google Maps' routing quality are both tracked as future refinements (see Phase 4 note above and Phase 6's ideas below).

## Phase 6 — Frontend

- [x] React (Vite) app (`frontend/`), deployed to GitHub Pages on push to `main`
- [x] `RouteFinder` component: From/To address inputs, calls `GET /route`, renders total time and an ordered list of legs — colored line bullets (reusing real MTA bullet colors) for rides, a walk icon for walk legs, real station names and "Take the {line} train from X to Y" phrasing
- [x] Click-to-fill suggestion chips for 8 popular NYC landmarks under each input, using full unambiguous names (a bare "Grand Central, NYC" geocodes to a Queens parkway — see Phase 5/`docs/ARCHITECTURE.md`)
- [x] Collapses zero-duration same-station walk legs in the display (a cross-complex transfer lands on a station's parent node, then a free hop to the specific child platform — two real graph edges that read as a nonsensical "walk to the same place twice" if shown as separate rows)
- [x] "Spinning up the database…" loading note once a request has been pending past a threshold, since Aurora Serverless scales to 0 ACU when idle and the first request after a quiet period pays a real resume cost

**Open decisions:** none blocking. Possible next steps if traffic/quality work continues: surfacing multiple geocoding candidates for ambiguous addresses instead of silently taking the first result, and a small benchmark script comparing `/route` against the Google Maps Directions API across a wider set of trips (discussed, not yet built).

---

## Suggested immediate next step

The core product loop is done: ingest → geocode → time-dependent A* search → HTTP API → frontend, deployed and verified against real data. The highest-leverage next step is likely **making `candidateStopCount` more than 1 affordable again** — batching or caching the per-expansion DB queries in `internal/routing/graph.go`'s `expand()` across a search (and across the from/to candidate pairs in `internal/api/route.go`) would let the search consider more than the single nearest platform at each end without reintroducing the request-timeout bug, which is likely the single biggest lever left on route quality versus Google Maps.
