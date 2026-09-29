.DEFAULT_GOAL := build

# Overridable: `export DATABASE_URL=...` before `make` to point elsewhere.
# Defaults to the docker-compose postgres (see ../docker-compose.yml and the
# root Makefile, which owns `up`/`down` for it).
DATABASE_URL ?= postgres://postgres:admin@localhost:5433/findmybike?sslmode=disable
export DATABASE_URL

.PHONY: build
build:
	go build -o ./bin/classifier ./cmd/classifier

.PHONY: run
run:
	./bin/classifier

.PHONY: test
test:
	go test ./...

.PHONY: fmt
fmt:
	gofmt -w .
