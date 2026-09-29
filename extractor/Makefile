.DEFAULT_GOAL := build

# Overridable: `export DATABASE_URL=...` before `make` to point elsewhere
# (e.g. a real deployment). Defaults to the docker-compose postgres (see
# ../docker-compose.yml and the root Makefile, which owns `up`/`down` for it).
DATABASE_URL ?= postgres://postgres:admin@localhost:5433/findmybike?sslmode=disable
export DATABASE_URL

.PHONY: build
build:
	go build -o ./bin/extract ./cmd/extract
	go build -o ./bin/merge ./cmd/merge
	go build -o ./bin/daterange ./cmd/daterange
	go build -o ./bin/migrate ./cmd/migrate
	go build -o ./bin/load ./cmd/load

.PHONY: test
test:
	go test ./...

migrate:
	./bin/migrate

run:
	./bin/extract

run-merge:
	./bin/merge

run-daterange:
	./bin/daterange

# usage: make run-watch DIR=../sourcing/raw/2026-09-20
run-watch:
	./bin/extract --watch $(DIR)

# usage: make run-load DIR=../sourcing/raw/2026-09-20 (or a single .html file)
run-load:
	./bin/load $(DIR)

fmt:
	gofmt -w .
