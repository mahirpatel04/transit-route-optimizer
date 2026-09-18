# Architecture Overview

## What it does

Ingests NYC subway GTFS (transit schedule) data from a public S3-hosted zip into Postgres on a weekly schedule, and serves a time-dependent A* route-finding API over it — geocoding two free-text NYC addresses, resolving each to a subway platform, and searching the ingested schedule for the fastest real trip. A small React frontend (`frontend/`) lets a person try it directly.

## Services used

| Service | Role |
|---|---|
| **AWS Lambda** (container image) | One function, two jobs: runs the weekly GTFS ingest (EventBridge-triggered) and serves the HTTP API (via a public Function URL) — branches on the incoming event shape |
| **Amazon Aurora Serverless v2 (PostgreSQL)** | Stores the transit data; scales to 0 ACU when idle (real cold-start latency on the first request after idle — the frontend surfaces a "spinning up the database" note past a threshold) |
| **Amazon EventBridge** | Weekly cron trigger for the ingest job |
| **Amazon ECR** | Hosts the Lambda's container image |
| **AWS Location Service** (Places) | Geocodes free-text addresses to lat/lon, bounded to a NYC bounding box; reached over a VPC interface endpoint, no internet egress needed |
| **Amazon VPC** (default VPC + S3 gateway endpoint + Location Service Places interface endpoint) | Private network so Lambda can reach Aurora and Location Service; free route to S3 for the GTFS download — the Lambda needs **no internet access at all** |
| **AWS CDK (Go)** | Infrastructure as code — defines all of the above |
| **Go** (`pgx`, `aws-lambda-go`) | Application language/runtime |
| **React (Vite)** | Frontend, deployed to GitHub Pages |

## Data flow — ingest

```text
EventBridge (weekly cron)
        │
        ▼
   Lambda (container image)
        │
        ├─ 1. Fetch GTFS zip from S3 (rrgtfsfeeds bucket, public)
        ├─ 2. Unzip in memory, parse 6 CSV files (stops, routes,
        │      calendar, calendar_dates, trips, stop_times)
        │      — header-name column lookup, not fixed index
        ├─ 3. Open one Postgres transaction:
        │      TRUNCATE all 6 tables → bulk INSERT via pgx CopyFrom
        │      → COMMIT (rollback on any failure)
        ▼
   Aurora Serverless v2 (Postgres)
```

## Data flow — route search

```text
Frontend (RouteFinder.jsx)
        │  GET /route?from=<address>&to=<address>
        ▼
   Lambda (same function/image as ingest, HTTP event path)
        │
        ├─ 1. Geocode both addresses (AWS Location Service Places,
        │      via the VPC interface endpoint), bounded to NYC
        ├─ 2. Resolve each geocoded point to its nearest subway
        │      platform (internal/routing.NearestStops)
        ├─ 3. Run time-dependent A* (internal/routing.FindRoute) from
        │      the nearest "from" platform to the nearest "to"
        │      platform, departing at the requested time (default:
        │      now) — expands real scheduled departures and transfer
        │      edges from Postgres as it searches
        ├─ 4. Price the address→platform and platform→address walks
        │      at each end (same pedestrian-pace model used for
        │      in-graph transfers) and prepend/append them as legs
        ├─ 5. Attach station names and line info to every leg
        ▼
   JSON response: total_time_seconds + ordered legs (walk/ride)
```

## The routing graph and its two fixed bugs

`internal/routing` models the search as: nodes = subway stops, edges = either a scheduled ride (from `stop_times`, respecting `calendar`/`calendar_dates` service-day filtering) or a walking transfer within/between station complexes. Two real bugs were found and fixed here after comparing results against Google Maps:

- **Search fan-out caused request timeouts.** The API originally tried the 3 nearest candidate stops at each end (9 total from/to combinations), each running an independent A* search that could issue hundreds of sequential DB round trips (`internal/routing/graph.go`'s `expand()`) — enough to blow the Lambda's request timeout under real traffic. Reduced to the single nearest candidate at each end (`candidateStopCount` in `internal/api/route.go`).
- **Cross-complex transfers were matched by exact `stop_name`, missing real transfers.** MTA doesn't always name the same physical transfer complex identically across lines — e.g. "Court Sq" (7/G trains) vs. "Court Sq-23 St" (E/M trains), ~90m apart. Requiring an exact string match silently dropped that transfer, making entire lines unreachable from some origins regardless of how many candidates were tried. Fixed in `internal/routing/transfers.go` by matching any two station complexes within a 250m radius, priced by actual walking distance instead of a flat cost.

## Why a single Lambda, not one-per-file / Step Functions

The whole ingest run (fetch + parse + insert 565K+ rows) takes seconds. Splitting per file would add orchestration complexity (Step Functions) for zero real benefit — no need for parallelism or per-file failure isolation at this scale.

## Why truncate-and-reload, not upsert/diff

GTFS is a full snapshot released periodically; IDs can be reused across feed versions in ways that make incremental diffing fragile. Truncate + reinsert inside one transaction is simple and correct: readers only ever see either the old complete dataset or the new one, never a partial mix (the transaction only commits if every table's insert succeeds).

## The entrypoint: one binary, two modes

`cmd/ingest/main.go` runs identically as a local CLI and as the Lambda handler — no separate binary/directory needed (Go doesn't allow two `func main()` in one package, so a second `cmd/` dir was the only other option, and was intentionally avoided). `main()` checks `AWS_LAMBDA_FUNCTION_NAME` (Lambda always sets this): if present, call `lambda.Start(run)`; if not, call `run()` directly and `log.Fatalf` on error. All ingest logic lives in `run(ctx) error`, shared by both paths.

## Networking: how Lambda reaches Aurora, S3, and Location Service with zero internet access

Lambda joins the same VPC as Aurora ("Lambda-in-VPC") to reach it over the normal Postgres wire protocol — this keeps `pgx`/`CopyFrom` unchanged versus rewriting to Aurora's Data API. The tradeoff: VPC-attached Lambda ENIs never get a public IP, regardless of which subnet type they sit in — a "public" subnet's internet gateway route doesn't help an ENI with no public IP to NAT through, so any VPC-attached Lambda needs an explicit private path to every AWS service it calls.

**S3 (GTFS download):** rather than paying for a NAT Gateway (~$32/mo) or NAT Instance (~$3-4/mo) to restore general internet access for one S3 download, the whole stack (Aurora, VPC, Lambda) lives in the **same region as the GTFS bucket** (`us-east-1`) and uses a **free S3 gateway VPC endpoint** instead. This only works because the endpoint is region-scoped — it doesn't cover cross-region S3 access, which is *why* the stack had to be same-region with the bucket in the first place (an earlier deploy in `us-east-2` failed for exactly this reason: the endpoint didn't cover the `us-east-1`-hosted bucket, causing every fetch to time out).

**Location Service (geocoding) — the NAT detour and its removal:** once geocoding needed real internet access (originally the public Nominatim API), a NAT instance and a new private subnet were added — five commits' worth of AMI-architecture, interface-detection, and iptables/nftables fixes, none of which should have been necessary. The actual fix was architectural, not a networking patch: switching the geocoder from Nominatim to **AWS Location Service (Places)**, which — like S3 — has a VPC endpoint (an *interface* endpoint this time, since Location Service isn't one of the handful of gateway-endpoint-eligible services). With that endpoint in place, the Lambda needs no internet access at all, so the NAT instance and private subnet were removed entirely; the Lambda sits in a public subnet (still with no public IP, which is fine — it never needs one) purely because that's where the default VPC's existing route tables/subnets already are.

## Secrets: plain Lambda env var, not Secrets Manager

Originally planned as a single Secrets Manager secret (cheaper than one-secret-per-field, since Secrets Manager bills per secret). Dropped after a real deploy failure: a VPC-attached Lambda with no NAT can't reach Secrets Manager's public API endpoint either — same root problem as the S3 case, but Secrets Manager has no free gateway-endpoint equivalent (only a paid interface endpoint, ~$7-8/month). Since the connection string needs no rotation, a plain Lambda environment variable (encrypted at rest by AWS by default) is the pragmatic choice — CDK builds `DATABASE_URL` at deploy time from the Aurora cluster's endpoint (a CDK token, resolved into a CloudFormation `Fn::Join` automatically via ordinary Go string concatenation) plus a password passed in via a `DB_MASTER_PASSWORD` env var at `cdk deploy` time.

## Infrastructure as code: CDK in Go, not Terraform

Chosen over Terraform to keep the whole project in one language — application code and infrastructure definitions are both Go, sharing the same tooling (compiler, module system, editor support) rather than splitting the codebase across Go and HCL.

Aurora itself was originally hand-clicked through the console (a deliberate Phase 2 choice, to learn the AWS console first) and later brought fully into CDK once a cross-region rebuild was needed anyway — so the whole stack (VPC lookups, security groups, Aurora cluster, Lambda, EventBridge rule) is now one CDK stack (`infra/infra.go`), reproducible via `cdk deploy` / destroyable via `cdk destroy`.

## Notable bugs hit and fixed along the way

- **CSV parsing by fixed column index** broke on real-world GTFS files with reordered/optional columns → rewrote to header-name lookup (`internal/gtfs/parse.go`'s `header` map).
- **`log.Fatalf` inside a library function** (`internal/gtfs/fetch.go`) would have killed a Lambda invocation ungracefully → changed to a returned `error`.
- **Re-running ingest hit `duplicate key value violates unique constraint`** → added `TRUNCATE ... CASCADE` + wrapped all inserts in one transaction.
- **Docker build context recursion**: `DockerImageCode_FromImageAsset("..")` copied `infra/cdk.out` (where CDK stages that very asset) into itself, causing `ENAMETOOLONG` → added `.dockerignore` excluding `infra/cdk.out`.
- **Lambda base image entrypoint mismatch**: `public.ecr.aws/lambda/provided:al2023`'s default entrypoint expects a handler-name argument (built for interpreted runtimes); a self-contained Go binary implementing the Runtime API loop needs `ENTRYPOINT` overridden to run it directly.
- **Password with a `^` character broke Postgres connection strings twice** — once in a manually-typed `.env.prod` (fixed via manual `%5E` encoding), once in CDK's generated `DATABASE_URL` (fixed properly via `net/url.QueryEscape`).
- **A NAT instance was added, then removed** — see the Networking section above. Five commits chasing AMI architecture/interface-detection/iptables issues turned out to be solving the wrong problem; switching geocoders let the whole NAT path be deleted.
- **Nominatim's bare "Grand Central, NYC" geocoded to a Queens parkway**, not Grand Central Terminal — the public Nominatim instance's result ranking favored a street/highway match over the intended point of interest for an ambiguous short query. Not fixed (the switch to AWS Location Service inherited the same class of ambiguity — a full address or landmark name like "Grand Central Terminal, NYC" resolves correctly on both geocoders); documented as a known limitation, and the frontend's suggestion chips use unambiguous full names.
- **Route search could time out under real traffic**: `candidateStopCount = 3` meant up to 9 independent A* searches per request, each capable of issuing hundreds of sequential DB round trips → reduced to 1 (see `internal/api/route.go`).
- **Cross-complex transfers were silently missing real transfers** due to exact `stop_name` matching — see the routing graph section above.
- **The route response had no walk legs at either end** and no station names, only raw stop IDs like `R15S` — fixed by adding real address↔platform walk legs (priced the same way as in-graph transfers) and a `stop_id → stop_name` lookup on every leg.

## What's explicitly out of scope (for now)

- CloudWatch Alarms / SNS alerting on ingest or API failure — logs are enough for a personal project.
- Aurora Data API — would avoid the VPC networking questions entirely, but requires rewriting the `pgx`/`CopyFrom` bulk-insert code.
- Per-file Lambdas / Step Functions — unnecessary orchestration for a sub-minute ingest job.
- A paid/higher-rate geocoder beyond AWS Location Service, or address-ambiguity handling (e.g. returning multiple candidates for the user to pick from) — not needed at current traffic.
- Making `candidateStopCount` runtime-configurable (e.g. a query param) instead of a compile-time constant, and batching/caching the DB queries inside `expand()` across candidate searches so more candidates can be tried again without timing out.
