.DEFAULT_GOAL := help

# Shared local dev database for the whole pipeline (extractor + classifier).
# Overridable: `export DATABASE_URL=...` before `make` to point elsewhere.
DATABASE_URL ?= postgres://postgres:admin@localhost:5433/findmybike?sslmode=disable
export DATABASE_URL

.PHONY: help
help:
	@echo "Common targets:"
	@echo "  make up        - start the local postgres (docker compose)"
	@echo "  make down      - stop it"
	@echo "  make migrate   - apply extractor's db migrations + seeds"
	@echo "  make reset-db  - wipe the local db and re-apply migrations from scratch"
	@echo "  make load DIR=path/to/html/files-or-dir - load extracted posts into the db"
	@echo "  make classify  - build and run the classifier triage GUI"
	@echo "  make build     - build every sub-project"
	@echo "  make test      - test every sub-project"
	@echo "  make fmt       - gofmt every sub-project"
	@echo "See README.md for the full pipeline walkthrough."

.PHONY: up
up:
	docker compose up -d
	@echo "waiting for postgres..."
	@until docker compose exec -T postgres pg_isready -U postgres > /dev/null 2>&1; do sleep 1; done
	@echo "postgres is ready"

.PHONY: down
down:
	docker compose down

.PHONY: build
build:
	$(MAKE) -C extractor build
	$(MAKE) -C classifier build

.PHONY: migrate
migrate:
	$(MAKE) -C extractor build migrate

# Drops and recreates the local findmybike database, then re-applies
# migrations from scratch. Destructive: all loaded posts and tags are lost --
# re-run `make load DIR=...` afterward to repopulate. Always targets the
# local docker-compose postgres via `docker compose exec`, regardless of any
# DATABASE_URL override.
.PHONY: reset-db
reset-db: up
	docker compose exec -T postgres psql -U postgres -c "DROP DATABASE IF EXISTS findmybike;"
	docker compose exec -T postgres psql -U postgres -c "CREATE DATABASE findmybike;"
	$(MAKE) migrate

# usage: make load DIR=sourcing/raw/2026-09-20
# DIR is resolved to an absolute path first since the extractor Makefile
# runs from extractor/, not the repo root.
.PHONY: load
load:
	$(MAKE) -C extractor build run-load DIR=$(abspath $(DIR))

.PHONY: classify
classify:
	$(MAKE) -C classifier build run

# classifier's suite needs a real Postgres (TEST_DATABASE_URL) for its
# integration test. A dedicated throwaway database is created and dropped
# around the run so `make test` can never point at (and wipe) the real
# findmybike database, however DATABASE_URL happens to be set.
.PHONY: test
test:
	cd domain && go test ./...
	$(MAKE) -C extractor test
	$(MAKE) up
	docker compose exec -T postgres psql -U postgres -c "DROP DATABASE IF EXISTS findmybike_test;" > /dev/null
	docker compose exec -T postgres psql -U postgres -c "CREATE DATABASE findmybike_test;" > /dev/null
	TEST_DATABASE_URL="postgres://postgres:admin@localhost:5433/findmybike_test?sslmode=disable" $(MAKE) -C classifier test; \
		status=$$?; \
		docker compose exec -T postgres psql -U postgres -c "DROP DATABASE IF EXISTS findmybike_test;" > /dev/null; \
		exit $$status

.PHONY: fmt
fmt:
	$(MAKE) -C extractor fmt
	$(MAKE) -C classifier fmt
	cd domain && gofmt -w .
