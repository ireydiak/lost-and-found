# find-my-bike

Sources, extracts, and triages Facebook posts about stolen/abandoned bikes
(the `velovolemtl` group). Three Go modules plus a TypeScript scraper, tied
together with a Go workspace and a root `Makefile`. See `INVESTIGATION.md`
for the architecture background and design history.

## Pipeline

```
sourcing/ (TS, Playwright)  -->  extractor/ (Go)  -->  Postgres  -->  classifier/ (Go)
     scrapes raw HTML            parses HTML into        shared         triage GUI:
     snapshots to disk           posts, loads them        database      tag each post
                                 into the database                      stolen/abandoned/other
```

- **`sourcing/`** — connects to a browser via Playwright and saves one raw
  HTML snapshot per scroll of the group's feed to `sourcing/raw*/`. Does no
  parsing.
- **`domain/`** — the shared `Post`/`Asset` types both `extractor` and
  `classifier` import. No logic, no dependencies.
- **`extractor/`** — parses saved HTML into `domain.Post` records, and owns
  the database: migrations, seeds, and the `cmd/load` bridge tool that
  upserts extracted posts into Postgres.
- **`classifier/`** — a small web GUI for tagging untagged posts
  (`stolen`/`abandoned`/`other`), reading and writing the same database.

`extractor/docs/DATABASE.md` is the authoritative schema/dedup reference.

## Prerequisites

- Go 1.26+ (the repo-root `go.work` ties `domain`, `extractor`, and
  `classifier` together for local development — no version tags or
  `replace` directives needed).
- Docker, for the local Postgres.

## Quickstart

```sh
make up                            # start Postgres (docker compose), wait for it to be ready
make migrate                       # apply extractor's db migrations + seed data
make load DIR=sourcing/raw/2026-09-20   # extract + load a directory of saved HTML snapshots
make classify                      # build and run the classifier triage GUI at :8080
```

None of this requires setting `DATABASE_URL` by hand — every Makefile in
this repo defaults it to the docker-compose Postgres
(`postgres://postgres:admin@localhost:5433/findmybike`). Export your own
`DATABASE_URL` before running `make` if you need to point at something else
(e.g. a real deployment); it overrides the default.

`make load` accepts either a directory of `.html` files or a single file.
Re-running it over the same file(s) is a safe no-op — every insert is
`ON CONFLICT DO NOTHING`, keyed on a hash of the file's raw contents.

## Other targets

Run `make help` for the full list. Highlights:

| Target | What it does |
|---|---|
| `make build` | Builds every sub-project's binaries. |
| `make test` | Runs every sub-project's test suite. `classifier`'s integration test spins up and tears down its own throwaway `findmybike_test` database — it never touches the real one. |
| `make fmt` | `gofmt`s every sub-project. |
| `make down` | Stops the local Postgres. |
| `make reset-db` | **Destructive.** Drops and recreates the local `findmybike` database, then re-applies migrations from scratch — all loaded posts and tags are lost. Re-run `make load DIR=...` afterward to repopulate. |

Each sub-project also has its own `Makefile` for project-specific tasks
(e.g. `extractor`'s `run-watch`, `run-merge`, `run-daterange` — see
`extractor/README.md`).

## Working directly in a sub-project

The root `Makefile` just delegates to each sub-project's own `Makefile` with
`make -C <dir> <target>`, forwarding `DATABASE_URL`. You can `cd` into
`extractor/` or `classifier/` and run their targets directly the same way —
useful when iterating on one project without rebuilding the others.
