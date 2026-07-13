# Deployment plan

*Written 2026-07-17. Prices checked against provider pages on that date — re-verify before committing, they move (Hetzner raised shared-vCPU prices ~30% in June 2026).*

## What we are deploying

| Component | Shape | Notes |
|---|---|---|
| `cmd/serve` | one small Go binary, static pages embedded | ~30 MB RAM, port from `PORT` |
| Postgres + **PostGIS** | tiny: ~200 shops, submissions, roles; < 100 MB for years | PostGIS is the hard requirement — not every managed Postgres ships it |
| `cmd/import-req`, `cmd/geocode` | occasional batch jobs | run from a laptop against the prod `DATABASE_URL`; no need to deploy them |

Runtime needs: HTTPS on a stable domain (OAuth redirect URIs + `Secure` cookies), outbound HTTPS to `nominatim.openstreetmap.org`, and these env vars: `DATABASE_URL`, `SESSION_SECRET`, `BASE_URL`, `ADMIN_EMAILS`, `GOOGLE_CLIENT_ID/SECRET`, `FACEBOOK_CLIENT_ID/SECRET`. Never set `DEV_AUTH` in production.

Prerequisite (small task, not yet done): a production Dockerfile that builds `cmd/serve` and `cmd/migrate` (the existing Dockerfile only runs migrations via goose).

## Options compared

Monthly cost for our actual workload (one always-on micro app + tiny Postgres):

| Option | Price/mo | Region for MTL users | DB story | Ops burden | Verdict |
|---|---|---|---|---|---|
| **Fly.io** (2 machines) | **~$4–5** — app shared-cpu-1x 256 MB ≈ $1.94 + Postgres machine w/ 1 GB volume ≈ $2.09 | **Montreal (yul)** | Unmanaged "Fly Postgres" (a machine + volume you own); daily volume snapshots, 5 kept. Their fully managed MPG starts ~$38 — skip it | Medium: you are the DBA, but snapshots + tiny data make that tolerable | **Recommended** |
| **Hetzner CX22 VPS** | **~€5.49** (+~€1 snapshots) | EU or US-Ashburn (~15–20 ms from MTL); no Canada | Run our existing `docker-compose` (postgis image) on the box | Highest: TLS, upgrades, backups are yours — but we already have the compose file | **Runner-up / most headroom** (2 vCPU, 4 GB, 20 TB traffic) |
| **Neon free + Fly app** | **~$2** | DB in AWS us-east; app yul | Managed serverless Postgres, PostGIS supported, 0.5 GB / 100 CU-hrs free; suspends when the quota is hit; cold starts | Low | **Budget-zero-ish variant**; fine for a soft launch, quota is the risk |
| **Railway** | ~$6–12 — $5 Hobby (includes $5 usage) + Postgres usage commonly $5+ alone | US-east closest | Deploy the `postgis` image as a service; simple | Lowest — best DX, GitHub push-to-deploy | Great DX, worst price ratio for the DB |
| **Render** | ~$14 — $7 web + $7 basic Postgres | US-east | Free Postgres is deleted after 30 days — trap; paid tier fine | Low | Outclassed at this size |
| **DigitalOcean droplet** | $6 (1 GB) | **Toronto** | Same self-managed compose as Hetzner, half the RAM | Same as Hetzner | Pick over Hetzner only if Canadian data residency matters |

Also considered: OVH (Beauharnois DC is basically Montreal — worth a look if data-in-Québec ever becomes a requirement), Supabase free (PostGIS included but free projects pause after a week of inactivity — wrong fit for an always-on map).

## Recommendation

**Fly.io, both machines in `yul`.** Best price for always-on (~$4–5/mo), the only option with compute *in Montreal*, automatic TLS + certs, Dockerfile-native deploys, and per-machine scaling later. The trade-off — self-managed Postgres — is small at our size: volume snapshots are automatic, and a nightly `pg_dump` shipped off-site (below) covers restore-anywhere.

**Fallback:** Hetzner CX22 with our existing `docker-compose.yaml` + Caddy if we ever want raw headroom (4 GB RAM) or to consolidate more projects on one box for the same money.

## Deploy runbook (Fly.io)

1. **Production Dockerfile** — multi-stage: `golang:1.26-alpine` build of `./cmd/serve` and `./cmd/migrate`, distroless/alpine runtime, `CMD ["serve"]`.
2. **Postgres first**
   - `fly postgres create --name shops-db --region yul --vm-size shared-cpu-1x --volume-size 1`
   - Verify PostGIS: `fly postgres connect -a shops-db` → `CREATE EXTENSION postgis;`. If the stock postgres-flex image lacks it, instead run a plain machine from `imresamu/postgis:17-3.5` with a 1 GB volume — same cost, and it matches local dev exactly.
3. **App**
   - `fly launch --no-deploy` (generates `fly.toml`; internal_port 8080, `region yul`, `min_machines_running = 1` to avoid auto-stop breaking sessionless OAuth callbacks).
   - `fly secrets set DATABASE_URL=... SESSION_SECRET=$(openssl rand -hex 32) ADMIN_EMAILS=... GOOGLE_CLIENT_ID=... GOOGLE_CLIENT_SECRET=... FACEBOOK_CLIENT_ID=... FACEBOOK_CLIENT_SECRET=... BASE_URL=https://<app>.fly.dev`
   - Migrations as a release step in `fly.toml`: `[deploy] release_command = "migrate"`.
   - `fly deploy`.
4. **Domain + OAuth** — either keep `<app>.fly.dev` or `fly certs add shops.example.com`; update `BASE_URL` and add `https://<domain>/auth/google/callback` and `/auth/facebook/callback` to the Google/Meta consoles.
5. **Data load** — from a laptop: `fly proxy 5433 -a shops-db`, then `DATABASE_URL=postgres://...@localhost:5433/shops make import-req && go run ./cmd/geocode`. Re-run on each REQ data refresh.
6. **Backups** — Fly volume snapshots are automatic (daily, 5 retained). Add a weekly off-site `pg_dump` via `fly proxy` from a laptop or a tiny GitHub Actions cron with the connection string as a secret. Test one restore before calling this done.
7. **Checks** — `[[http_service.checks]]` on `GET /api/shops`; optionally a free UptimeRobot/Kuma ping on the public URL.

## Runbook sketch (Hetzner fallback)

CX22 + Ubuntu LTS → `docker compose up -d` (existing file, add the app service) → Caddy container for TLS with the domain → UFW allow 80/443 only → nightly `pg_dump` to Hetzner Storage Box or B2 → `unattended-upgrades` on. Everything else (env vars, OAuth, imports) identical.

## Cost summary

| Path | Year one |
|---|---|
| Fly.io (recommended) | ~$50–60 |
| Hetzner CX22 | ~€66–78 |
| Neon free + Fly app | ~$24 until traffic outgrows the free DB |

## Explicitly deferred

- CI/CD pipeline (deploys are `fly deploy` by hand for now)
- Rate limiting on write routes, CSRF token hardening if cross-origin API use ever appears
- A 30-second admin-lookup cache if request volume ever makes the per-request role query visible
