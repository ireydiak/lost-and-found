# Classifier GUI — Design

## Purpose

A GUI for admins to review scraped Facebook posts and assign them one or more of three tags — `stolen`, `abandoned`, `other` — writing only into the `tags`/`posts_categories` tables of the shared database that `extractor` owns and migrates (see `extractor/docs/DATABASE.md`). `classifier` never runs migrations and never writes to `posts_meta`/`posts_details`/`posts_assets`.

## Access model

Runs locally, single-admin, for now — no authentication, no deployment concerns. It's expected to grow into a shared tool on a web server later; the architecture is chosen so that transition doesn't require a rewrite.

## Approach: server-rendered Go web app

Stdlib `net/http` + `html/template`. No JS framework, no frontend build step. The core interaction — view a post, check some boxes, submit — is a plain HTML form POST; Go's `ServeMux` (1.22+ pattern matching) is sufficient for the handful of routes v1 needs, no router library required.

This matches the minimal-dependency style already established across the repo's Go projects (`extractor` has no non-essential dependencies; `classifier` currently has none at all), and scales to "shared web tool" by adding an auth layer and deploying — not by replacing the UI paradigm.

**Alternatives considered and rejected for v1:**
- **Go JSON API + separate JS frontend** — buys smoother interactions (e.g. preloading the next image), at the cost of two codebases and a build step, for a workflow (check boxes, submit) where that responsiveness gain is marginal.
- **A different language/stack** (Python, Node) — every other Go project in this repo already reads/writes the same Postgres schema; introducing a second language for the one piece that reads it buys nothing.

## Project structure

```
classifier/
  cmd/classifier/main.go        # entrypoint: reads DATABASE_URL + APP_ENV, starts the HTTP server
  internal/
    triage/
      post.go                    # Post/Asset structs shared by handlers + templates
      queue.go                   # SQL: fetch the next untagged post + its assets, count remaining
      tag.go                     # SQL: save selected tags for a post
    web/
      handlers.go                 # HTTP handlers: GET / , POST /tag
      templates/
        triage.html               # the triage screen
  go.mod
  Makefile
  README.md
```

Mirrors `extractor`'s `internal/fbpost` + thin `cmd/` shape (domain logic separated from the thin entrypoint), even though the two projects share no code. `triage` (domain/DB logic) is kept separate from `web` (HTTP layer) so a different interface onto the same queue — a CLI, an API — wouldn't need to change `triage` or drag `net/http` along with it. Not building anything beyond the web UI now; this is just where the seam naturally falls.

## Data flow / routes

**`GET /`** — fetch and render the next untagged post.

```sql
SELECT pd.html_hash, pd.author, pd.title, pd.description, pd.date
FROM posts_details pd
LEFT JOIN posts_categories pc ON pc.html_hash = pd.html_hash
WHERE pc.html_hash IS NULL
ORDER BY pd.date DESC
LIMIT 1
```

Semantically the same set difference as the `posts_details EXCEPT posts_categories` access pattern already documented in `extractor/docs/DATABASE.md`, expressed as `LEFT JOIN ... WHERE NULL` so it can be ordered and limited to one row directly.

Assets are fetched with a second, simple query — `SELECT url, asset_type FROM posts_assets WHERE html_hash = $1` — no fan-out risk, since a single post realistically has 0-4 assets. A `SELECT COUNT(*)` over the same untagged set gives a "N posts remaining" figure for the template. If no untagged post exists, the page renders an "All caught up" empty state instead of a post + form.

**`POST /tag`** — submits `html_hash` (hidden field) and one or more checked `tag_id` values.

```sql
INSERT INTO posts_categories (tag_id, html_hash) VALUES ($1, $2), ...
ON CONFLICT DO NOTHING
```

`ON CONFLICT DO NOTHING` makes a double-submit harmless — relevant once this is shared and two admins could grab the same post at once. If zero tags are checked, nothing is inserted and the same post is redisplayed with an inline "select at least one tag" error — validated server-side, not via client-side JS, since there's no JS in this design. On success, redirects (`303 See Other`) back to `GET /`, which naturally serves the next untagged post.

## Templates / UI

A single template: author, date, title (when present), description, a loop over assets rendering `<img>` tags, and a form with a hidden `html_hash` field, three checkboxes (`Stolen` / `Abandoned` / `Other`), and a "Save & Next" submit button. The "all caught up" state reuses the same template, substituting the empty-state message for the post block and form. Plain HTML with enough CSS to be usable — no styling framework, no JS.

## Error handling

- DB connection failure at startup → fail fast with a clear log message (`log.Fatal`), matching `extractor`'s `cmd/migrate`.
- Query/scan errors during a request → logged server-side; response rendered according to `APP_ENV` (see below).
- Empty tag submission → handled as described above (redisplay with inline error), not a 500.

**`APP_ENV`-based error detail:** a single `APP_ENV` environment variable, read once at startup alongside `DATABASE_URL`. When `APP_ENV=development`, the error-rendering path includes the actual error message and stack trace in the HTTP response. Any other value (or unset — the assumed production-safe default) renders a generic 500 page with no internal detail. This is a single conditional in one shared error-rendering helper, not a broader config system.

## Testing

- Pure logic (e.g. "was at least one tag selected") gets ordinary Go unit tests.
- The SQL queries in `internal/triage` are integration-tested against a real, throwaway Postgres instance — the same way the migrations themselves were verified (spin up a container, run migrations, exercise the queries, tear down). Mocking the database here would mostly test that the mock behaves like the mock; the real risk is in the SQL itself.
