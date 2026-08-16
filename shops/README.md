# Shops

A collaborative map of Montreal bike shops — Go web app + Postgres/PostGIS.

## Prerequisites

- Go 1.26+
- Docker (with Compose)

## Quick start

```bash
export DATABASE_URL="postgres://postgres:admin@localhost:5432/shops?sslmode=disable"

make up      # postgres + PostGIS in Docker; migrations and seeds run automatically
SESSION_SECRET=dev DEV_AUTH=1 ADMIN_EMAILS=you@example.com make serve
```

Open <http://localhost:8080>. With `DEV_AUTH=1` the sign-in button leads to a
local dev login (any email works — use one from `ADMIN_EMAILS` to get the admin
tools). Never set `DEV_AUTH` outside local development.

## Loading shop data

Download the REQ dump (browser only) from
[Données Québec](https://www.donneesquebec.ca/recherche/dataset/entreprises),
save it as `data/JeuDonnees.zip`, then:

```bash
make import-req   # parse the dump, upsert shops + addresses
make geocode      # resolve addresses lacking a location via Nominatim (~1 req/s)
```

Both need `DATABASE_URL` exported. Re-running is safe.

## Tests

```bash
make test
```

Integration tests need the database up and are skipped when `DATABASE_URL` is unset.

## Make targets

| Target | What it does |
|---|---|
| `make up` / `make down` | Start / stop the Docker stack (postgres + migrations) |
| `make clean` | Stop the stack **and delete the database volume** — full reset |
| `make serve` | Run the web server on `:8080` (env vars below) |
| `make import-req` / `make geocode` | Load shop data — see above |
| `make migrate` | Run migrations + seeds with a local `goose` |
| `make test` | Run the test suite |
| `make deploy` | Deploy to Fly.io |

## Environment variables (`cmd/serve`)

| Variable | Required | Notes |
|---|---|---|
| `DATABASE_URL` | yes | Postgres connection string |
| `SESSION_SECRET` | production | Signs session cookies; random per-start if unset |
| `ADMIN_EMAILS` | recommended | Comma-separated bootstrap admins; more can be granted at runtime from `/admin.html` |
| `DEV_AUTH` | dev only | `1` enables the local dev login. Never set outside local development |
| `PORT` | no | Listen port, default `8080` |

Full production config (OAuth credentials, `BASE_URL`, etc.) and the deploy
runbook: see [docs/deployment-plan.md](docs/deployment-plan.md) and
[docs/fly-postgres.md](docs/fly-postgres.md).

## Misc

Create a new migration:

```bash
goose -dir db/migrations create my_change sql
```

Connect to the local database (requires `pgcli`, `brew install pgcli`):

```bash
pgcli "postgres://postgres:admin@localhost:5432/shops?sslmode=disable"
```
