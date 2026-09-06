.PHONY: up down reset logs build migrate-dev migrate-prod ingest-dev ingest-prod

# Purpose: start the local Postgres + migrate containers in the background.
# Target:  local Docker Postgres (.env / docker-compose.yml).
up:
	docker compose up -d

# Purpose: stop the local containers without deleting data.
# Target:  local Docker Postgres.
down:
	docker compose down

# Purpose: wipe the local DB volume and rebuild containers from scratch, rerunning migrations.
# Target:  local Docker Postgres.
reset:
	docker compose down -v
	docker compose up -d --build

# Purpose: rebuild and (re)start the local containers, e.g. after a Dockerfile change.
# Target:  local Docker Postgres.
build:
	docker compose up -d --build

# Purpose: tail the migrate container's logs.
# Target:  local Docker Postgres.
logs:
	docker compose logs -f migrate

# Purpose: apply pending goose migrations.
# Target:  local Docker Postgres (.env).
migrate-dev:
	@set -a; . ./.env; set +a; goose -dir migrations postgres "$$DATABASE_URL" up

# Purpose: apply pending goose migrations.
# Target:  Aurora Serverless v2 (.env.prod).
migrate-prod:
	@set -a; . ./.env.prod; set +a; goose -dir migrations postgres "$$DATABASE_URL" up

# Purpose: run the GTFS ingest job (truncates all tables, then re-loads them).
# Target:  local Docker Postgres (.env).
ingest-dev:
	@set -a; . ./.env; set +a; go run cmd/ingest/main.go

# Purpose: run the GTFS ingest job (truncates all tables, then re-loads them).
# Target:  Aurora Serverless v2 (.env.prod).
ingest-prod:
	@set -a; . ./.env.prod; set +a; go run cmd/ingest/main.go
