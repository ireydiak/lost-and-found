# Classifier Database

## Tables

### Posts Meta

Used to register posts for the first time, before data transformation and extraction. Encapsulates basic information about the raw, as-sourced HTML — nothing here depends on extraction logic, so it can be computed the instant a file is saved.

| Column name | Type | Description |
| --------------- | --------------- | --------------- |
| sourced_at | utc timestamp not null | when the post was sourced |
| filename | varchar not null | relative path to the raw sourced HTML file |
| html_hash | CHAR(64) PK | SHA-256 hash of the raw HTML file's bytes. Identifies this specific *ingestion* (has this exact clipboard/scrape content already been saved before) — it does NOT identify the same real-world post across separate scrapes, since Facebook's markup carries per-request noise (CDN signing tokens, tracking params, generated element IDs) that differs even when the underlying post is identical. See "Deduplication" below for how the same real-world post is recognized across multiple ingestions. |
| facebook_id | varchar null | Facebook's own post/permalink ID, when extractable from the raw HTML (present in ~96% of sourced posts in practice — see Deduplication). Deliberately NOT unique-constrained here: the same real-world post can legitimately be re-sourced across multiple sessions, producing multiple `posts_meta` rows that share the same `facebook_id`. |

### Posts Details

Detailed information about the posts: author, title, description, pictures, etc. — populated once a `posts_meta` row has been successfully parsed.

| Column name | Type | Description |
| --------------- | --------------- | --------------- |
| html_hash | CHAR(64) PK, FK -> posts_meta.html_hash | identifies which raw ingestion this was extracted from |
| author | varchar not null | name of the original post author (or the literal "Anonymous participant" for posts with no linkable profile — see Deduplication) |
| title | varchar null | title of the post, when the source uses a template with a distinct title line. Most posts have no separate title (their whole text is just `description`) — nullable rather than required. Not part of `dedup_key` — see Deduplication. |
| description | text null | post description |
| date | timestamp not null | the post's own real-world date (when the bike was stolen/found/abandoned), resolved from whatever relative or absolute date format Facebook rendered at sourcing time |
| dedup_key | varchar not null | fallback identity for recognizing the same real-world post across ingestions when `facebook_id` is unavailable — see Deduplication |
| processed_at | utc timestamp not null | when the item was processed |

### Posts Assets

| Column name | Type | Description |
| --------------- | --------------- | --------------- |
| asset_id | BIGINT PK | surrogate key |
| html_hash | CHAR(64) FK -> posts_details.html_hash | relation to the post |
| url | varchar not null | link to the asset |
| asset_type | enum | type of the asset (picture, video) |

### Tags

Helps categorize posts. A post can carry more than one tag (e.g. a theft-status tag alongside a future neighborhood or bike-type tag) — kept as a many-to-many relation rather than a single column on `posts_details` for that reason.

| Column name | Type | Description |
| --------------- | --------------- | --------------- |
| tag_id | BIGINT PK | surrogate key |
| name | varchar not null, UNIQUE | name of the tag (e.g. "stolen", "abandoned", "other") |

### Posts Categories

Links `tags` records to posts.

| Column name | Type | Description |
| --------------- | --------------- | --------------- |
| tag_id | BIGINT FK -> tags.tag_id | tag unique identifier |
| html_hash | CHAR(64) FK -> posts_details.html_hash | post unique identifier |

Primary key: `(tag_id, html_hash)` — prevents the same tag being attached to the same post more than once.

## Deduplication

`html_hash` identifies an *ingestion*, not a real-world post — the same post sourced in two different sessions produces two different `posts_meta`/`posts_details` rows with different `html_hash` values. Recognizing that they're the same underlying post is a separate step, run after extraction, in priority order:

1. **`facebook_id` match.** If both rows have a non-null `facebook_id` and it's equal, they're the same post. This is the reliable signal when available — empirically present for ~96% of sourced posts (a `/posts/<id>` permalink or a `fbid=` photo-attachment reference), and unlike any hash of extracted fields, it doesn't drift with how Facebook happened to render the date at scrape time.

2. **`dedup_key` match**, when `facebook_id` is absent on either row. Computed as `hash(author, date)` — deliberately **excludes `title` and `description`**: authors can and do edit their posts after the fact (e.g. adding "MAJ : vélo retrouvé" once a bike is found), and the original content is what this key is meant to recognize, not track edit history. `author + date` alone was measured against a real corpus (688 posts) and found 5 genuine same-author/same-day collisions (~0.7%) that this key would incorrectly merge — an accepted trade-off, since the alternative (including `title`/`description`) causes false *negatives* on every edited post, which is considered worse.

   **Special case — `"Anonymous participant"`:** this literal string is a fallback label for any post with no linkable profile (see `posts_details.author`), so it doesn't identify a real individual and same-day collisions among different anonymous posters are expected, not exceptional. For these, `dedup_key` additionally mixes in a random salt generated once at insert time and stored with the row: `hash(author, date, random_salt)`. This makes the key (and thus this fallback path) effectively never match another row by coincidence, so anonymous posts are always treated as distinct here — true duplicates among them are only caught by `facebook_id`, when present.

## Access Patterns

Which data access patterns does the proposed database schema address?

### Unprocessed queue

AP: Retrieve posts that were not processed.
Query: Simple set difference between `posts_meta` and `posts_details`.

```sql
SELECT html_hash FROM posts_meta
EXCEPT
SELECT html_hash FROM posts_details
```


### Processed queue

AP: Retrieve posts that were processed.
Query: The processed queue is equivalent to the `posts_details` table.

```sql
SELECT html_hash FROM posts_details
```


### Untagged posts

AP: Retrieve posts missing classification.
Query: Set difference between `posts_details` and `posts_categories`.

```sql
SELECT html_hash FROM posts_details
EXCEPT
SELECT html_hash FROM posts_categories
```

### Retrieve post information

AP: Retrieve detailed posts information, including assets and tags.
Query: tags and assets are each pre-aggregated in their own CTE before joining — `posts_categories` and `posts_assets` are independent one-to-many relations off the same post, so joining both directly in one query would fan out into a cross product (a post with 2 tags and 3 assets producing 6 rows before aggregation); aggregating each side separately first avoids that entirely, rather than relying on `DISTINCT` inside `ARRAY_AGG` to paper over it. The final joins are `LEFT JOIN`, not `INNER JOIN` — a meaningful fraction of real posts have no picture at all (~11-12%, confirmed against actual sourced data) and any post can be legitimately untagged (that's exactly what the "Untagged posts" access pattern above depends on) — an `INNER JOIN` would silently drop those posts from the result set entirely.

```sql
WITH post_tags AS (
  SELECT pc.html_hash, ARRAY_AGG(t.name) AS tags
  FROM posts_categories pc
  INNER JOIN tags t ON t.tag_id = pc.tag_id
  GROUP BY pc.html_hash
),
post_assets AS (
  SELECT pa.html_hash, ARRAY_AGG(jsonb_build_object('url', pa.url, 'asset_type', pa.asset_type)) AS assets
  FROM posts_assets pa
  GROUP BY pa.html_hash
)
SELECT
  pd.html_hash,
  pd.author,
  pd.title,
  pd.date,
  pm.sourced_at,
  pd.processed_at,
  COALESCE(pt.tags, ARRAY[]::varchar[]) AS tags,
  COALESCE(pa.assets, ARRAY[]::jsonb[]) AS assets
FROM posts_details pd
INNER JOIN posts_meta pm ON pm.html_hash = pd.html_hash
LEFT JOIN post_tags pt ON pt.html_hash = pd.html_hash
LEFT JOIN post_assets pa ON pa.html_hash = pd.html_hash
```
