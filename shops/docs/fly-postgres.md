# Postgres on Fly.io: connecting and moving data

*Runbook for the production database (`shops-db`, a plain Fly machine running
`imresamu/postgis:17-3.5` with a volume — the same image as local dev). It is
reachable only on Fly's private network as `shops-db.internal:5432`; nothing is
exposed publicly.*

## Connecting

### From your laptop (the everyday path)

Open a WireGuard tunnel in one terminal and leave it running:

```bash
fly proxy 5433:5432 -a shops-db
```

Then connect through `localhost:5433` with any client. Without a local `psql`,
use the one inside the dev Docker container (reaches the host tunnel via
`host.docker.internal`) or `pgcli`:

```bash
docker compose exec -e PGPASSWORD=<password> postgres \
  psql -h host.docker.internal -p 5433 -U postgres -d shops

pgcli "postgres://postgres:<password>@localhost:5433/shops?sslmode=disable"
```

### Recovering the password

The password lives only in Fly secrets, which cannot be read back
(`fly secrets list` shows digests). To recover it, read the environment inside
the database machine:

```bash
fly ssh console -a shops-db -C "printenv POSTGRES_PASSWORD"
```

The app's full connection string is the `DATABASE_URL` secret on the app:
`postgres://postgres:<password>@shops-db.internal:5432/shops?sslmode=disable`.

### Directly inside the machine (no tunnel)

For quick one-off queries or health checks:

```bash
fly ssh console -a shops-db -C "pg_isready -U postgres"
fly ssh console -a shops-db -C "psql -U postgres -d shops -c 'SELECT count(*) FROM shops'"
```

## Schema migrations

Migrations run **automatically on every deploy**: `fly.toml` sets
`release_command = "migrate"`, which runs the `cmd/migrate` binary (goose
migrations + seeds) from the freshly built image *before* the new version takes
traffic. A failing migration aborts the deploy and the old version keeps
running.

To run migrations manually without a deploy (rarely needed):

```bash
fly ssh console -a lost-and-found-silver-voice-4700 -C "migrate"
```

Or from a laptop through the tunnel, with a local goose:

```bash
DATABASE_URL="postgres://postgres:<password>@localhost:5433/shops?sslmode=disable" make migrate
```

## Moving data

### Local → production (what the initial load used)

Prefer copying the local database over re-running the import + geocode in
production: it preserves the geocoded coordinates and spares Nominatim ~200
duplicate requests.

```bash
# 1. dump the data tables from local dev
docker compose exec -T postgres pg_dump -U postgres -d shops --data-only \
  -t tags -t addresses -t shops -t shops_tags -t socials > /tmp/proddata.sql

# 2. with the fly proxy running, reset the seeded tags and restore
docker compose exec -T -e PGPASSWORD=<password> postgres \
  psql -h host.docker.internal -p 5433 -U postgres -d shops \
  -c "TRUNCATE tags RESTART IDENTITY CASCADE;"
docker compose exec -T -e PGPASSWORD=<password> postgres \
  psql -h host.docker.internal -p 5433 -U postgres -d shops \
  -q -v ON_ERROR_STOP=1 < /tmp/proddata.sql
```

The dump's `COPY` statements carry explicit ids through identity columns and
end with `setval` calls, so sequences stay in sync. Do **not** dump
`submissions`/`users` from dev — production owns those.

### Refreshing from a new REQ dump

The importer can also run directly against production (it upserts on the NEQ
key, so existing shops update in place). New shops arrive without coordinates;
the geocoder fills them:

```bash
# with the proxy running
export DATABASE_URL="postgres://postgres:<password>@localhost:5433/shops?sslmode=disable"
make import-req
make geocode        # only touches addresses with no location; ~1 req/s
```

### Production → local (debugging with real data)

```bash
docker compose exec -T -e PGPASSWORD=<password> postgres \
  pg_dump -h host.docker.internal -p 5433 -U postgres -d shops > /tmp/prod-snapshot.sql
```

Restore into a scratch local database, not your dev one, if you want to keep
dev state.

## Backups

Two layers — both matter, because Fly volumes are host-pinned and unreplicated:

1. **Automatic volume snapshots**: daily, 5 retained. List and restore:
   ```bash
   fly volumes list -a shops-db
   fly volumes snapshots list <volume-id>
   fly volumes create pgdata --snapshot-id <snapshot-id> --region yyz -a shops-db
   ```
   Restoring creates a *new* volume; start a new machine on it and repoint
   nothing — the app finds whatever machine owns `shops-db.internal`.
2. **Off-site logical dump** (survives a Fly-wide problem): with the proxy
   running, `pg_dump` as above and store the file outside Fly. The whole
   database is well under 1 MB; a weekly cron (laptop or GitHub Actions with
   the connection string as a secret) is plenty. Test a restore once before
   trusting it.

## Gotchas learned the hard way

- **256 MB is not enough to `CREATE EXTENSION postgis`** — the OOM-killer eats
  the backend mid-statement (symptom: `driver: bad connection` during
  migrations). The machine runs 512 MB; don't scale it back down.
- Fly's `fly postgres create` (postgres-flex) image **has no PostGIS** — only
  their ~$38/mo Managed Postgres does. That's why `shops-db` is a plain machine
  with the `imresamu/postgis` image; `fly postgres attach/connect` conveniences
  don't apply to it.
- The Postgres data directory must be a **subdirectory of the volume**
  (`PGDATA=/var/lib/postgresql/data/pgdata`): the volume root contains
  `lost+found`, which makes initdb refuse to initialize.
