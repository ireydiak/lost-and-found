.DEFAULT_GOAL := build

.PHONY: build
build:
	go build -o ./bin/extract ./cmd/extract
	go build -o ./bin/merge ./cmd/merge
	go build -o ./bin/daterange ./cmd/daterange
	go build -o ./bin/migrate ./cmd/migrate

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

fmt:
	gofmt -w .
