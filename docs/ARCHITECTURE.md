# Architecture Overview

## What it does

Ingests NYC subway GTFS (transit schedule) data from a public S3-hosted zip, loads it into a Postgres database, and runs that ingestion automatically on a weekly schedule in AWS. Groundwork for a future time-dependent A* route-finding feature.

## Services used

| Service | Role |
|---|---|
| **AWS Lambda** (container image) | Runs the ingest job on a schedule, no server to manage |
| **Amazon Aurora Serverless v2 (PostgreSQL)** | Stores the transit data; scales to 0 ACU when idle |
| **Amazon EventBridge** | Weekly cron trigger for the Lambda |
| **Amazon ECR** | Hosts the Lambda's container image |
| **Amazon VPC** (default VPC + S3 gateway endpoint) | Private network so Lambda can reach Aurora; free route to S3 for the GTFS download |
| **AWS CDK (Go)** | Infrastructure as code — defines all of the above |
| **Go** (`pgx`, `aws-lambda-go`) | Application language/runtime |

## Data flow

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

## Why a single Lambda, not one-per-file / Step Functions

The whole ingest run (fetch + parse + insert 565K+ rows) takes seconds. Splitting per file would add orchestration complexity (Step Functions) for zero real benefit — no need for parallelism or per-file failure isolation at this scale.

## Why truncate-and-reload, not upsert/diff

GTFS is a full snapshot released periodically; IDs can be reused across feed versions in ways that make incremental diffing fragile. Truncate + reinsert inside one transaction is simple and correct: readers only ever see either the old complete dataset or the new one, never a partial mix (the transaction only commits if every table's insert succeeds).

## The entrypoint: one binary, two modes

`cmd/ingest/main.go` runs identically as a local CLI and as the Lambda handler — no separate binary/directory needed (Go doesn't allow two `func main()` in one package, so a second `cmd/` dir was the only other option, and was intentionally avoided). `main()` checks `AWS_LAMBDA_FUNCTION_NAME` (Lambda always sets this): if present, call `lambda.Start(run)`; if not, call `run()` directly and `log.Fatalf` on error. All ingest logic lives in `run(ctx) error`, shared by both paths.

## Networking: how Lambda reaches both Aurora and the public internet

Lambda joins the same VPC as Aurora ("Lambda-in-VPC") to reach it over the normal Postgres wire protocol — this keeps `pgx`/`CopyFrom` unchanged versus rewriting to Aurora's Data API. The tradeoff: VPC-attached Lambdas lose default internet access, and the GTFS zip is fetched from S3, a public endpoint.

**Key design point:** rather than paying for a NAT Gateway (~$32/mo) or NAT Instance (~$3-4/mo) just to restore general internet access for one S3 download, the whole stack (Aurora, VPC, Lambda) lives in the **same region as the GTFS bucket** (`us-east-1`) and uses a **free S3 gateway VPC endpoint** instead. This only works because the endpoint is region-scoped — it doesn't cover cross-region S3 access, which is *why* the stack had to be same-region with the bucket in the first place (an earlier deploy in `us-east-2` failed for exactly this reason: the endpoint didn't cover the `us-east-1`-hosted bucket, causing every fetch to time out).

## Secrets: plain Lambda env var, not Secrets Manager

Originally planned as a single Secrets Manager secret (cheaper than one-secret-per-field, since Secrets Manager bills per secret). Dropped after a real deploy failure: a VPC-attached Lambda with no NAT can't reach Secrets Manager's public API endpoint either — same root problem as the S3 case, but Secrets Manager has no free gateway-endpoint equivalent (only a paid interface endpoint, ~$7-8/month). Since the connection string needs no rotation, a plain Lambda environment variable (encrypted at rest by AWS by default) is the pragmatic choice — CDK builds `DATABASE_URL` at deploy time from the Aurora cluster's endpoint (a CDK token, resolved into a CloudFormation `Fn::Join` automatically via ordinary Go string concatenation) plus a password passed in via a `DB_MASTER_PASSWORD` env var at `cdk deploy` time.

## Infrastructure as code: CDK in Go, not Terraform

Chosen partly for resume/learning value: meaningful Terraform proficiency needs modules, remote state, and multi-env setup to look like more than "used it once," which is disproportionate setup for this project's size. CDK in Go doubles as Go practice instead, using the same language as the application.

Aurora itself was originally hand-clicked through the console (a deliberate Phase 2 choice, to learn the AWS console first) and later brought fully into CDK once a cross-region rebuild was needed anyway — so the whole stack (VPC lookups, security groups, Aurora cluster, Lambda, EventBridge rule) is now one CDK stack (`infra/infra.go`), reproducible via `cdk deploy` / destroyable via `cdk destroy`.

## Notable bugs hit and fixed along the way

- **CSV parsing by fixed column index** broke on real-world GTFS files with reordered/optional columns → rewrote to header-name lookup (`internal/gtfs/parse.go`'s `header` map).
- **`log.Fatalf` inside a library function** (`internal/gtfs/fetch.go`) would have killed a Lambda invocation ungracefully → changed to a returned `error`.
- **Re-running ingest hit `duplicate key value violates unique constraint`** → added `TRUNCATE ... CASCADE` + wrapped all inserts in one transaction.
- **Docker build context recursion**: `DockerImageCode_FromImageAsset("..")` copied `infra/cdk.out` (where CDK stages that very asset) into itself, causing `ENAMETOOLONG` → added `.dockerignore` excluding `infra/cdk.out`.
- **Lambda base image entrypoint mismatch**: `public.ecr.aws/lambda/provided:al2023`'s default entrypoint expects a handler-name argument (built for interpreted runtimes); a self-contained Go binary implementing the Runtime API loop needs `ENTRYPOINT` overridden to run it directly.
- **Password with a `^` character broke Postgres connection strings twice** — once in a manually-typed `.env.prod` (fixed via manual `%5E` encoding), once in CDK's generated `DATABASE_URL` (fixed properly via `net/url.QueryEscape`).

## What's explicitly out of scope (for now)

- CloudWatch Alarms / SNS alerting on ingest failure — logs are enough for a personal project.
- Aurora Data API — would avoid the VPC networking questions entirely, but requires rewriting the `pgx`/`CopyFrom` bulk-insert code.
- Per-file Lambdas / Step Functions — unnecessary orchestration for a sub-minute job.
