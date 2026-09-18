# transit-route-optimizer

A time-dependent A* subway router for NYC: give it two addresses, it geocodes them, finds the fastest real subway trip using live GTFS schedule data, and returns walk/ride legs with station names and lines. Full pipeline — GTFS ingestion, routing engine, HTTP API, and a React frontend — deployed on AWS.

**Live app:** [mahirpatel04.github.io/transit-route-optimizer](https://mahirpatel04.github.io/transit-route-optimizer/)

## Stack

| | |
|---|---|
| Backend | Go (`pgx`, `aws-lambda-go`) |
| Database | Aurora Serverless v2 (Postgres) |
| Compute | AWS Lambda (container image) — one function serves both the weekly GTFS ingest and the HTTP API |
| Geocoding | AWS Location Service, reached over a VPC interface endpoint (no NAT, no internet egress) |
| Infra | AWS CDK (Go) |
| Frontend | React (Vite), deployed to GitHub Pages |

## How it works

1. **Ingest** (`cmd/ingest`, weekly via EventBridge): fetch the GTFS zip from S3, parse 6 CSV files, truncate + bulk-load into Postgres in one transaction.
2. **Route search** (`GET /route?from=<address>&to=<address>`): geocode both addresses, resolve each to a subway platform, run a time-dependent A* search (`internal/routing`) over real scheduled departures, transfers, and service-day calendars.
3. **Two search strategies**, toggled by `?optimize=true`:
   - **Default — nearest platform.** Minimizes walking distance at each end.
   - **Optimized — multi-target A*.** Considers the 3 nearest platforms at the destination in a single search (a multi-goal heuristic, not 3 separate searches), so a farther-to-walk platform on a faster line can win. Never slower than the default, sometimes 20-30% faster.

See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for the full data flow and key engineering decisions, and [`docs/ROADMAP.md`](docs/ROADMAP.md) for phase-by-phase build history.

## Local development

```bash
make up              # local Postgres + migrations
make ingest-dev       # run the ingest job locally
```

`.env` needs `DATABASE_URL` and `PLACE_INDEX_NAME` (any placeholder value works for ingest-only work).

## Deploying

```bash
make migrate-prod && make ingest-prod   # against Aurora, needs .env.prod

cd infra
export DB_MASTER_PASSWORD='...'
cdk deploy
```

## Project structure

```text
cmd/ingest/       entrypoint — dual-mode CLI/Lambda, also the API's Lambda handler
internal/gtfs/     GTFS fetch/parse
internal/db/       Postgres access (pgx)
internal/geocode/  AWS Location Service geocoder
internal/routing/  time-dependent A* search, transfers, service-day resolution
internal/api/      HTTP handlers (GET /route, GET /ingest-time)
migrations/        goose schema migrations
infra/             AWS CDK app (Go)
frontend/          React client
docs/              architecture + roadmap
```
