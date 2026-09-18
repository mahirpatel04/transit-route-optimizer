# Architecture

## What it does

Ingests NYC subway GTFS data into Postgres weekly, and serves a time-dependent A* route-finding API over it — geocode two addresses, resolve each to a subway platform, search the schedule for the fastest real trip. A React frontend (`frontend/`) drives it directly.

## Services

| Service | Role |
|---|---|
| **AWS Lambda** (container image) | One function, two jobs: weekly GTFS ingest (EventBridge) and the HTTP API (Function URL) — branches on event shape |
| **Aurora Serverless v2 (Postgres)** | Transit data; scales to 0 ACU when idle |
| **EventBridge** | Weekly ingest cron |
| **AWS Location Service (Places)** | Geocodes addresses to lat/lon, bounded to NYC, over a VPC interface endpoint |
| **VPC** (default VPC + S3 gateway endpoint + Location Service interface endpoint) | Lambda needs **no internet access at all** |
| **AWS CDK (Go)** | All of the above, one stack (`infra/infra.go`) |

## Data flow — ingest

```text
EventBridge (weekly) → Lambda → fetch GTFS zip from S3 → parse 6 CSVs
  → one Postgres transaction: TRUNCATE all tables → bulk INSERT (pgx.CopyFrom) → COMMIT
```

## Data flow — route search

```text
Frontend → GET /route?from=&to=&optimize= → Lambda (HTTP path)
  1. Geocode both addresses (AWS Location Service)
  2. Resolve to nearest subway platform(s) — 1 candidate each by default,
     3 destination candidates if optimize=true
  3. Time-dependent A* search (internal/routing.FindRouteMultiTarget)
  4. Price + attach the address↔platform walks at both ends
  5. Attach station names and line info to every leg
  → JSON: total_time_seconds + ordered legs
```

## Routing engine

`internal/routing` models the graph as: nodes = subway stops, edges = scheduled rides (from `stop_times`, filtered by `calendar`/`calendar_dates` service-days) or walking transfers between station complexes (free within the same parent station, distance-priced between complexes within 250m).

**Two search strategies** (`GET /route?...&optimize=true`):
- **Default:** nearest destination platform only, minimizes walking.
- **Optimized:** `FindRouteMultiTarget` runs one A* search across the 3 nearest destination platforms, using a multi-goal heuristic (min estimated time across all targets — equivalent to a zero-cost edge from every target to one virtual destination). No extra DB load vs. the default; never slower, sometimes 20-30% faster.

**Two real bugs found by comparing results against Google Maps:**
- *Candidate fan-out timeouts:* trying 3 candidates at both ends meant up to 9 full searches per request, each issuing hundreds of DB round trips — exceeded the Lambda timeout. Fixed by capping candidates to 1 (later recovered via multi-target search, above, without the extra cost).
- *Cross-complex transfers matched by exact `stop_name`:* MTA names the same physical complex differently across lines (e.g. "Court Sq" vs. "Court Sq-23 St", ~90m apart), so exact-match silently dropped real transfers. Fixed by matching any two complexes within 250m instead, priced by walking distance.

## Key design decisions

- **One Lambda, not per-file/Step Functions** — the whole ingest run takes seconds; no orchestration needed.
- **Truncate + reload, not upsert/diff** — GTFS is a periodic full snapshot; IDs can be reused across versions, making diffing fragile. One transaction means readers never see a partial mix.
- **One binary, two modes** — `cmd/ingest/main.go` runs as both a local CLI and the Lambda handler, branching on `AWS_LAMBDA_FUNCTION_NAME`.
- **No NAT Gateway/Instance** — Lambda reaches S3 via a free gateway VPC endpoint (same-region as the GTFS bucket) and AWS Location Service via a VPC interface endpoint. A NAT instance was added, then removed, when geocoding briefly used the public Nominatim API instead — switching geocoders eliminated the internet dependency entirely rather than working around it.
- **Plain env var for `DATABASE_URL`, not Secrets Manager** — a VPC-attached Lambda with no NAT can't reach Secrets Manager's public endpoint either, and the connection string needs no rotation.
- **CDK in Go, not Terraform** — one language for app code and infra.

## Out of scope (for now)

- CloudWatch Alarms/SNS alerting — logs are enough for a personal project.
- Aurora Data API, per-file Lambdas/Step Functions — unnecessary for this scale.
- A paid/higher-rate geocoder, or surfacing multiple geocoding candidates for ambiguous addresses.
- Making candidate counts runtime-configurable beyond the `optimize` toggle.
