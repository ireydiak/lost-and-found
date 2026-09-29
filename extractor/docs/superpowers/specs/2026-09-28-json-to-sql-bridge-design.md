# JSON-to-SQL bridge — Design

## Purpose

`extractor` produces posts as JSON; `classifier` needs them in the Postgres database it reads from. Building the missing "load extractor's output into the database" tool surfaced three real gaps that needed resolving first: `extractor` and `classifier` had no shared vocabulary for what a post is, `fbpost.Post` was missing fields the schema requires (`FacebookID`, and multi-asset support), and roughly 1% of currently-sourced files yield more than one "post" due to Facebook comments being mis-detected as posts by the run-pairing extraction logic.

## Comment mis-detection (resolved as a data-integrity fix, not a schema change)

Scanning the full 715-file corpus found 6 files (~0.8%) yielding 2 posts each. Inspection of one showed both "posts" sharing the exact same photo URL, with the second's author/text being a reply-shaped comment (`"Epic bicycle story."`) on the first's real theft report — a comment carries its own author/text/timestamp, matching the same structural signature `findPosts` looks for. This is extraction picking up a comment as if it were an independent post, not two genuine reports.

**Resolution:** the loader keeps only the first post from each file's `[]domain.Post`, logging a warning (with the filename) whenever more than one is found, so the discard stays visible rather than silent. Given this policy, `posts_details.html_hash` remains a 1:1 primary key exactly as already migrated — no schema change needed. If genuine multi-post-per-file support is needed later (e.g. after fixing comment-detection at the extraction level), that's the point at which a surrogate key on `posts_details` would become worth adding.

## Shared domain module

A new module, `domain/`, sibling to `extractor` and `classifier`, containing only the shared data shape — no extraction logic, no DB code, no web code:

```go
// domain/post.go
package domain

import "time"

type AssetType string

const (
	AssetTypePicture AssetType = "picture"
	AssetTypeVideo   AssetType = "video"
)

type Asset struct {
	URL  string
	Type AssetType
}

type Post struct {
	HTMLHash    string    // SHA-256 of the raw source file's bytes
	FacebookID  *string   // Facebook's own post/permalink ID, when extractable
	Author      string
	Title       *string   // always nil for now -- title extraction deferred
	Description *string
	Date        time.Time
	Assets      []Asset
}
```

A `go.work` at the repo root:

```
go 1.26.1

use (
	./classifier
	./domain
	./extractor
)
```

lets both `extractor` and `classifier` import `domain` by local resolution during development — no version tags, no `replace` directives, no third-party build tool.

**Why a third module, not a direct import between the two projects:** `extractor` importing `classifier` would be backwards (the pipeline's first stage depending on its last-stage consumer). `classifier` importing `extractor` directly would drag in extraction-specific internals (selectors, date-parsing) that `classifier` has no business depending on, just to get a `Post` type. A minimal, stable third module avoids both.

**`fbpost.Post` and `fbpost.Asset` are deleted** — `fbpost.ExtractPosts` returns `[]domain.Post` directly. The old `Picture *string` field becomes `Assets []domain.Asset` (a 0-or-1-length slice today, correctly modeling the schema's actual multi-asset support even though extraction only ever finds one picture per post currently). `PosInset` (extraction's old output-position field) is dropped entirely — its own doc comment already said it wasn't a stable identifier; now that `HTMLHash`/`FacebookID` provide real identity, it no longer earns its keep. This is a breaking change to `cmd/extract`'s JSON output shape — acceptable, since nothing outside this project currently consumes it.

`classifier`'s own `triage.Post`/`triage.Asset` are deleted in favor of `domain.Post`/`domain.Asset` — `triage`'s queries already select exactly this shape, and the HTML template needs no changes since field names match. `Tag` stays in `classifier`; tagging is classifier's own concern, not something extraction produces.

## `facebook_id` extraction

New in `extractor/internal/fbpost`, following the same two-tier fallback pattern already used for description/picture extraction: scan every `<a href>` in a post's scope, preferring the more official post-permalink ID over a photo-attachment ID.

```go
var (
	postIDRegex = regexp.MustCompile(`/posts/(\d+)`)
	fbidRegex   = regexp.MustCompile(`[?&]fbid=(\d+)`)
)

func extractFacebookID(scope *goquery.Selection) *string {
	if id := firstMatch(scope, postIDRegex); id != nil {
		return id
	}
	return firstMatch(scope, fbidRegex)
}
```

Verified against the real 715-file corpus before being considered correct (empirically measured earlier at ~96% combined coverage across both patterns), not just trusted from the regex alone.

## Bridge tool: `extractor/cmd/load`

Lives in `extractor`, which already owns migrations for the tables it writes. Accepts a file or a directory, mirroring `cmd/extract`'s existing dual-mode handling exactly: `os.Stat` + `IsDir()`, `filepath.Glob(dir/*.html)` + `sort.Strings` for deterministic order, skip-and-log (not abort) on a single file's failure.

For each file:

1. Compute SHA-256 of the raw file bytes → `html_hash`.
2. Call `fbpost.ExtractPostsFromFile(path)` → `[]domain.Post`.
3. Zero posts → skip, log.
4. Keep only `posts[0]`; `len(posts) > 1` → log a warning naming the file (see Comment mis-detection above).
5. Upsert `posts_meta` (`html_hash`, `filename`, `sourced_at` = file mtime, `facebook_id`) — `ON CONFLICT (html_hash) DO NOTHING`, making re-running the loader over the same directory a safe no-op.
6. Upsert `posts_details` (`html_hash`, `author`, `title`, `description`, `date`, `dedup_key`, `processed_at` = now). `dedup_key` is computed here, in the loader — not in `domain`, since it's a persistence-layer concept (how the DB disambiguates a post), not part of what a post fundamentally is. Computed as `hash(author, date)`, with the random-salt carve-out for `"Anonymous participant"` already documented in `extractor/docs/DATABASE.md`. Same `ON CONFLICT DO NOTHING` idempotency.
7. Upsert each asset into `posts_assets` — `ON CONFLICT (html_hash, url) DO NOTHING`, matching the existing unique constraint.
8. Print a summary: files processed, posts loaded, files skipped (zero posts), files with discarded extras.

## Testing

- `extractFacebookID` verified against the real corpus, same rigor as every other extraction rule in this project.
- `cmd/load` integration-tested against a real, throwaway Postgres: load a file, confirm the rows; load the same file again, confirm it's a no-op; load a known multi-post file, confirm only the first post persists and a warning is logged for the discard.
- Full `go build`/`go test` across all three modules (`extractor`, `classifier`, `domain`) after the `domain.Post` migration, to confirm nothing broke in the switch.
