# transit-route-optimizer

Ingests NYC subway GTFS (transit schedule) data into Postgres, automated weekly on AWS. Groundwork for a future time-dependent A* route-finding feature.

## Stack

| | |
|---|---|
| Language | Go (`pgx`, `aws-lambda-go`) |
| Database | Aurora Serverless v2 (Postgres), local Docker Postgres for dev |
| Compute | AWS Lambda (container image), triggered weekly by EventBridge |
| Infra | AWS CDK (Go) — `infra/` |

## How it works

`cmd/ingest` fetches the GTFS zip from S3, parses its 6 CSV files (header-name column lookup, not fixed index), then truncates and bulk-inserts all 6 tables inside one Postgres transaction (`pgx.CopyFrom`). The same binary runs both as a local CLI and as the Lambda handler — it checks `AWS_LAMBDA_FUNCTION_NAME` at startup to decide which.

See [`docs/ROADMAP.md`](docs/ROADMAP.md) for the full phase-by-phase design history, decisions, and tradeoffs (NAT vs. S3 endpoint, Secrets Manager vs. plain env var, etc.).

## Local development

```bash
make up              # start local Postgres + run migrations
make ingest-dev       # run the ingest job against local Postgres
make reset            # wipe local DB and rebuild from scratch
```

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
cmd/ingest/          entrypoint (dual-mode: local CLI + Lambda handler)
internal/gtfs/        fetch, unzip, parse GTFS files
internal/db/          Postgres inserts (pgx.CopyFrom)
internal/config/      DATABASE_URL loading
migrations/           goose schema migrations
infra/                AWS CDK app (Go)
docs/ROADMAP.md        phase-by-phase design history and decisions
```
