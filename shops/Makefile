.DEFAULT_GOAL := build

.PHONY: build
build: install
	go build -o ./bin/migrate ./cmd/migrate

.PHONY: clean
clean:
	docker compose down -v

.PHONY: down
down:
	docker compose down

.PHONY: geocode
geocode:
	go run ./cmd/geocode

.PHONY: install
install:
	go mod download
	go mod tidy

.PHONY: serve
serve:
	go run ./cmd/serve

.PHONY: migrate
migrate:
	goose -dir db/migrations postgres "$(DATABASE_URL)" up
	goose -dir db/seeds -table goose_db_version_seeds postgres "$(DATABASE_URL)" up

.PHONY: up
up:
	docker compose up -d

.PHONY: test
test:
	go test ./...

.PHONY: deploy
deploy:
	fly deploy

.PHONY: import-req
import-req:
	go run ./cmd/import-req -zip data/JeuDonnees.zip

.PHONY: import-req-dry
import-req-dry:
	go run ./cmd/import-req -zip data/JeuDonnees.zip -dry-run
