.PHONY: up down reset logs build psql

up:
	docker compose up -d

down:
	docker compose down

reset:
	docker compose down -v
	docker compose up -d --build

build:
	docker compose up -d --build

logs:
	docker compose logs -f migrate

psql:
	@set -a; . ./.env; set +a; psql "$$DATABASE_URL"