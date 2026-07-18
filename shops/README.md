# Shops

A collaborative map of Montreal bike shops. Shop data comes from the
[Registre des entreprises du Québec](https://www.donneesquebec.ca/recherche/dataset/entreprises)
(CAE code 6542 — bicycle retail), is geocoded against OpenStreetMap's Nominatim,
and is served by a small Go web app (Leaflet map, list and detail views, and a
moderated edit/suggestion flow behind OAuth).

## Prerequisites

- Go 1.26+
- Docker (with Compose)
- [`goose`](https://github.com/pressly/goose) only if you want to run migrations
  outside Docker (`make up` runs them for you)

## Quick start

```bash
export DATABASE_URL="postgres://postgres:admin@localhost:5432/shops?sslmode=disable"

make up      # postgres + PostGIS in Docker; migrations and seeds run automatically
SESSION_SECRET=dev DEV_AUTH=1 ADMIN_EMAILS=you@example.com make serve
```

Open <http://localhost:8080>. With `DEV_AUTH=1` the sign-in button leads to a
local dev login (any email works — use one from `ADMIN_EMAILS` to get the admin
tools). Never set `DEV_AUTH` outside local development.

## Importing the shop data

1. Download the REQ dump (browser only) from
   [Données Québec](https://www.donneesquebec.ca/recherche/dataset/entreprises)
   and save it as `data/JeuDonnees.zip`.
2. Import and geocode:

```bash
make import-req       # parse the dump, upsert shops + addresses
make import-req-dry   # preview to data/import-preview.csv, no database writes
make geocode          # resolve addresses lacking a location via Nominatim (~1 req/s)
```

Both need `DATABASE_URL` exported. Re-running is safe: the importer upserts on
the NEQ natural key and the geocoder only touches addresses without coordinates.

## Make targets

| Target | What it does |
|---|---|
| `make up` / `make down` | Start / stop the Docker stack (postgres + migrations) |
| `make clean` | Stop the stack **and delete the database volume** — full reset |
| `make serve` | Run the web server on `:8080` (env vars below) |
| `make import-req` | Import shops from `data/JeuDonnees.zip` |
| `make import-req-dry` | Same, but writes a preview CSV instead of the database |
| `make geocode` | Fill missing `addresses.location` via Nominatim |
| `make migrate` | Run migrations + seeds with a local `goose` |
| `make build` | Compile the migrate binary to `./bin/` |
| `make install` | `go mod download` + `tidy` |
| `make test` | Run the test suite |

## Commands

| Command | Purpose |
|---|---|
| `cmd/serve` | Web server: map/list/detail pages, JSON + GeoJSON API, OAuth, moderation queue |
| `cmd/import-req` | REQ dump importer |
| `cmd/geocode` | Batch geocoder (Nominatim, rate-limited, postal-code sanity checks) |
| `cmd/migrate` | Programmatic migrations + seeds (used by the Docker migrate service) |
| `cmd/mintsession` | Dev tool: prints a valid session cookie for an email (`SESSION_SECRET=... go run ./cmd/mintsession you@example.com`) |

## Environment variables (`cmd/serve`)

| Variable | Required | Notes |
|---|---|---|
| `DATABASE_URL` | yes | Postgres connection string |
| `SESSION_SECRET` | production | Signs session cookies; random per-start if unset (sessions won't survive restarts) |
| `BASE_URL` | production | Public origin, e.g. `https://shops.example.com`; defaults to `http://localhost:8080` |
| `ADMIN_EMAILS` | recommended | Comma-separated bootstrap admins; more can be granted at runtime from `/admin.html` |
| `DEV_AUTH` | dev only | `1` enables the local dev login (localhost only, any email signs in) |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | production | Google OAuth app credentials |
| `FACEBOOK_CLIENT_ID` / `FACEBOOK_CLIENT_SECRET` | production | Facebook OAuth app credentials |
| `PORT` | no | Listen port, default `8080` |

## Pages

| URL | View |
|---|---|
| `/` | Interactive map (clusters, search, status filter) |
| `/shops.html` | List of every shop, including ones without coordinates |
| `/shop.html?id=N` | Shop details; suggest an edit (users) or edit directly (admins) |
| `/suggest.html` | Propose a brand-new shop, with an optional map pin |
| `/admin.html` | Moderation queue, history, and admin role management |

## Tests

```bash
DATABASE_URL="postgres://postgres:admin@localhost:5432/shops?sslmode=disable" go test ./...
```

Integration tests (submissions, roles) need the database up and are skipped
when `DATABASE_URL` is unset.

## Misc

Create a new migration:

```bash
goose -dir db/migrations create my_change sql
```

Connect to the local database (requires `pgcli`, `brew install pgcli`):

```bash
pgcli "postgres://postgres:admin@localhost:5432/shops?sslmode=disable"
```

Deployment: see [docs/deployment-plan.md](docs/deployment-plan.md).
