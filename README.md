# transit-route-optimizer

Ingests NYC subway GTFS (transit schedule) data into Postgres, automated weekly on AWS, and serves time-dependent A* route search over it via a small HTTP API and React frontend. Give it two NYC addresses and a `GET /route` request returns the fastest real subway trip between them — geocoded, walk legs included, station names and lines spelled out.

## Stack

| | |
|---|---|
| Language | Go (`pgx`, `aws-lambda-go`), React (Vite) frontend |
| Database | Aurora Serverless v2 (Postgres), local Docker Postgres for dev |
| Compute | AWS Lambda (container image) — GTFS ingest (weekly, via EventBridge) and the HTTP API (via a public Function URL) share one Lambda/binary |
| Geocoding | AWS Location Service (Places), reached over a VPC interface endpoint — no NAT, no internet egress needed |
| Infra | AWS CDK (Go) — `infra/` |

## How it works

**Ingest:** `cmd/ingest` fetches the GTFS zip from S3, parses its 6 CSV files (header-name column lookup, not fixed index), then truncates and bulk-inserts all 6 tables inside one Postgres transaction (`pgx.CopyFrom`).

**Routing:** `GET /route?from=<address>&to=<address>` geocodes both addresses (AWS Location Service, bounded to NYC), resolves each to its nearest subway stop, and runs a time-dependent A* search (`internal/routing`) over the ingested schedule — respecting real service-day calendars, transfer costs between stations, and scheduled departure times. The response includes the walk from your typed address to the first platform, every ride/walk leg with station names and line, and the walk from the last platform to your destination.

The same Lambda binary serves both jobs and both event sources — it checks `AWS_LAMBDA_FUNCTION_NAME` at startup, then branches on the incoming event shape (EventBridge cron vs. an HTTP request) to decide whether to run the ingest job or dispatch to the API mux (`internal/api`).

**Frontend:** a small React app (`frontend/`) with a from/to route finder, deployed to GitHub Pages.

See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for a deeper look at how the pieces fit together, and [`docs/ROADMAP.md`](docs/ROADMAP.md) for the full phase-by-phase design history, decisions, and tradeoffs (NAT vs. S3/Location Service endpoints, geocoder choice, transfer-matching bugs found and fixed, etc.).

## Local development

```bash
make up              # start local Postgres + run migrations
make ingest-dev       # run the ingest job against local Postgres
make reset            # wipe local DB and rebuild from scratch
```

`.env` needs both `DATABASE_URL` and `PLACE_INDEX_NAME` — `internal/config.Load()` is shared between the ingest job and the API, so it requires both even though ingest itself never geocodes. `PLACE_INDEX_NAME` only matters for exercising `/route` locally; any placeholder value is fine for `make ingest-dev`.

## Deploying / running against Aurora

Requires `.env.prod` (gitignored) with a `DATABASE_URL` pointing at Aurora.

```bash
make migrate-prod     # apply schema migrations to Aurora
make ingest-prod      # run the ingest job against Aurora
```

Infrastructure (Aurora, VPC, Lambda, EventBridge) is defined in `infra/infra.go` and deployed via CDK:

```bash
cd infra
export DB_MASTER_PASSWORD='...'   # Aurora master password
export DEV_IP=$(curl -s https://checkip.amazonaws.com)  # optional: allow your IP to psql in
cdk deploy
```

## Project structure

```text
cmd/ingest/           entrypoint (dual-mode: local CLI + Lambda handler; also the API's Lambda entry)
internal/gtfs/         fetch, unzip, parse GTFS files
internal/db/           Postgres inserts (pgx.CopyFrom) and queries
internal/config/       DATABASE_URL / PLACE_INDEX_NAME loading
internal/geocode/      AWS Location Service + Nominatim geocoder implementations
internal/routing/      time-dependent A* search, transfer graph, service-day resolution
internal/api/          HTTP handlers (GET /route, GET /ingest-time)
migrations/            goose schema migrations
infra/                 AWS CDK app (Go)
frontend/              React (Vite) client, deployed to GitHub Pages
docs/ARCHITECTURE.md   how the pieces fit together and why
docs/ROADMAP.md        phase-by-phase design history and decisions
```
