.DEFAULT_GOAL := build

.PHONY: build
build:
	go build -o ./bin/extract ./cmd/extract

run:
	./bin/extract

fmt:
	go fmt ./cmd/extract/*.go
	gofmt -w ./cmd/extract/extract.go
