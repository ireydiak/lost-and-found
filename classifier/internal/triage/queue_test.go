package triage

import (
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// testDB connects to TEST_DATABASE_URL and creates the minimal schema these
// tests need. It's self-contained rather than depending on extractor's
// migrations directly, keeping classifier's test suite independent -- the
// trade-off is that this schema subset must be kept in sync by hand with
// extractor/db/migrations if those tables ever change shape.
func testDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	schema := `
		DROP TABLE IF EXISTS posts_categories;
		DROP TABLE IF EXISTS posts_assets;
		DROP TABLE IF EXISTS posts_details;
		DROP TABLE IF EXISTS posts_meta;
		DROP TABLE IF EXISTS tags;
		DROP TYPE IF EXISTS post_asset_type;

		CREATE TABLE posts_meta (
			html_hash CHAR(64) PRIMARY KEY,
			filename VARCHAR(500) NOT NULL,
			sourced_at TIMESTAMPTZ NOT NULL,
			facebook_id VARCHAR(50)
		);

		CREATE TABLE posts_details (
			html_hash CHAR(64) PRIMARY KEY REFERENCES posts_meta(html_hash) ON DELETE CASCADE,
			author VARCHAR(255) NOT NULL,
			title VARCHAR(500),
			description TEXT,
			date TIMESTAMPTZ NOT NULL,
			dedup_key VARCHAR(255) NOT NULL,
			processed_at TIMESTAMPTZ NOT NULL
		);

		CREATE TYPE post_asset_type AS ENUM ('picture', 'video');

		CREATE TABLE posts_assets (
			asset_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
			html_hash CHAR(64) NOT NULL REFERENCES posts_details(html_hash) ON DELETE CASCADE,
			url TEXT NOT NULL,
			asset_type post_asset_type NOT NULL
		);

		CREATE TABLE tags (
			tag_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
			name VARCHAR(100) NOT NULL UNIQUE
		);

		CREATE TABLE posts_categories (
			tag_id BIGINT NOT NULL REFERENCES tags(tag_id) ON DELETE RESTRICT,
			html_hash CHAR(64) NOT NULL REFERENCES posts_details(html_hash) ON DELETE CASCADE,
			PRIMARY KEY (tag_id, html_hash)
		);

		INSERT INTO tags (name) VALUES ('stolen'), ('abandoned'), ('other');
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		t.Fatalf("failed to set up test schema: %v", err)
	}

	t.Cleanup(func() {
		db.Exec(`
			DROP TABLE IF EXISTS posts_categories;
			DROP TABLE IF EXISTS posts_assets;
			DROP TABLE IF EXISTS posts_details;
			DROP TABLE IF EXISTS posts_meta;
			DROP TABLE IF EXISTS tags;
			DROP TYPE IF EXISTS post_asset_type;
		`)
		db.Close()
	})

	return db
}

// facebookID may be "" for a post with no extractable Facebook ID (NULL in
// posts_meta), matching real data -- see extractor/docs/DATABASE.md.
func insertTestPost(t *testing.T, db *sql.DB, htmlHash, author string, date time.Time, facebookID string) {
	t.Helper()
	var fbID any
	if facebookID != "" {
		fbID = facebookID
	}
	_, err := db.Exec(
		`INSERT INTO posts_meta (html_hash, filename, sourced_at, facebook_id) VALUES ($1, 'test.html', NOW(), $2)`,
		htmlHash, fbID,
	)
	if err != nil {
		t.Fatalf("failed to insert posts_meta: %v", err)
	}
	_, err = db.Exec(
		`INSERT INTO posts_details (html_hash, author, description, date, dedup_key, processed_at)
		 VALUES ($1, $2, 'a test post', $3, $4, NOW())`,
		htmlHash, author, date, "dedup-"+htmlHash,
	)
	if err != nil {
		t.Fatalf("failed to insert posts_details: %v", err)
	}
	_, err = db.Exec(
		`INSERT INTO posts_assets (html_hash, url, asset_type) VALUES ($1, 'https://example.com/a.jpg', 'picture')`,
		htmlHash,
	)
	if err != nil {
		t.Fatalf("failed to insert posts_assets: %v", err)
	}
}

func TestQueueFlow(t *testing.T) {
	db := testDB(t)

	now := time.Now().UTC().Truncate(time.Second)
	insertTestPost(t, db, "hash-older", "Alice", now.Add(-24*time.Hour), "")
	insertTestPost(t, db, "hash-newer", "Bob", now, "")

	tags, err := AllTags(db)
	if err != nil {
		t.Fatalf("AllTags: %v", err)
	}
	if len(tags) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(tags))
	}
	var stolenID int64
	for _, tg := range tags {
		if tg.Name == "stolen" {
			stolenID = tg.ID
		}
	}
	if stolenID == 0 {
		t.Fatalf("expected a 'stolen' tag to exist")
	}

	count, err := CountUntagged(db)
	if err != nil {
		t.Fatalf("CountUntagged: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 untagged posts, got %d", count)
	}

	// NextUntagged orders by date DESC, so the newer post ("Bob") should
	// come first.
	post, err := NextUntagged(db, nil)
	if err != nil {
		t.Fatalf("NextUntagged: %v", err)
	}
	if post == nil {
		t.Fatal("expected a post, got nil")
	}
	if post.Author != "Bob" {
		t.Fatalf("expected Bob (newer post) first, got %s", post.Author)
	}
	if len(post.Assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(post.Assets))
	}
	if post.Assets[0].Type != "picture" {
		t.Fatalf("expected asset_type 'picture', got %s", post.Assets[0].Type)
	}

	// Tag Bob's post and confirm the queue advances to Alice's.
	if err := SaveTags(db, post.HTMLHash, []int64{stolenID}); err != nil {
		t.Fatalf("SaveTags: %v", err)
	}

	count, err = CountUntagged(db)
	if err != nil {
		t.Fatalf("CountUntagged after tagging: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 untagged post after tagging, got %d", count)
	}

	next, err := NextUntagged(db, nil)
	if err != nil {
		t.Fatalf("NextUntagged after tagging: %v", err)
	}
	if next == nil || next.Author != "Alice" {
		t.Fatalf("expected Alice's post next, got %+v", next)
	}

	// Double-submitting the same tag for the same post must not error
	// (ON CONFLICT DO NOTHING).
	if err := SaveTags(db, post.HTMLHash, []int64{stolenID}); err != nil {
		t.Fatalf("expected duplicate SaveTags to be a no-op, got error: %v", err)
	}
}

// TestDuplicatePosts covers the same real-world post being sourced across
// multiple scrape sessions -- each producing its own posts_meta/posts_details
// row (by design, see extractor/docs/DATABASE.md), but sharing a
// facebook_id. The queue must treat these as one post, not one per
// ingestion.
func TestDuplicatePosts(t *testing.T) {
	db := testDB(t)

	now := time.Now().UTC().Truncate(time.Second)
	insertTestPost(t, db, "hash-dup-older", "Carol", now.Add(-1*time.Hour), "fb-dup-1")
	insertTestPost(t, db, "hash-dup-newer", "Carol", now, "fb-dup-1")
	insertTestPost(t, db, "hash-solo", "Dave", now.Add(-2*time.Hour), "")

	count, err := CountUntagged(db)
	if err != nil {
		t.Fatalf("CountUntagged: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 untagged posts (the duplicate pair collapsed into one), got %d", count)
	}

	post, err := NextUntagged(db, nil)
	if err != nil {
		t.Fatalf("NextUntagged: %v", err)
	}
	// html_hash is CHAR(64) in Postgres, which space-pads these short test
	// IDs -- trim before comparing. Real html_hash values are always
	// exactly 64 hex characters, so this padding never occurs in practice.
	if post == nil || strings.TrimSpace(post.HTMLHash) != "hash-dup-newer" {
		t.Fatalf("expected the newer duplicate ingestion (hash-dup-newer) as the group's representative, got %+v", post)
	}

	tags, err := AllTags(db)
	if err != nil {
		t.Fatalf("AllTags: %v", err)
	}
	var stolenID int64
	for _, tg := range tags {
		if tg.Name == "stolen" {
			stolenID = tg.ID
		}
	}

	if err := SaveTags(db, post.HTMLHash, []int64{stolenID}); err != nil {
		t.Fatalf("SaveTags: %v", err)
	}

	// Without propagation, hash-dup-older would still have no row in
	// posts_categories, so it would resurface later as if it were a fresh,
	// distinct untagged post -- exactly the duplicate-viewing bug being
	// fixed here.
	var siblingTagged bool
	if err := db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM posts_categories WHERE html_hash = 'hash-dup-older' AND tag_id = $1)`,
		stolenID,
	).Scan(&siblingTagged); err != nil {
		t.Fatalf("checking sibling tag: %v", err)
	}
	if !siblingTagged {
		t.Fatal("expected tagging one duplicate ingestion to propagate to its sibling")
	}

	count, err = CountUntagged(db)
	if err != nil {
		t.Fatalf("CountUntagged after tagging: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 untagged post after tagging the duplicate group, got %d", count)
	}

	next, err := NextUntagged(db, nil)
	if err != nil {
		t.Fatalf("NextUntagged after tagging: %v", err)
	}
	if next == nil || next.Author != "Dave" {
		t.Fatalf("expected Dave's post next, got %+v", next)
	}
}

// TestSkip covers "skip this post for now" -- it must move the queue past
// the post without tagging it (CountUntagged unaffected), and FilenameFor
// must resolve the source file recorded for it.
func TestSkip(t *testing.T) {
	db := testDB(t)

	now := time.Now().UTC().Truncate(time.Second)
	insertTestPost(t, db, "hash-a-older", "Alice", now.Add(-1*time.Hour), "")
	insertTestPost(t, db, "hash-b-newer", "Bob", now, "")

	post, err := NextUntagged(db, nil)
	if err != nil {
		t.Fatalf("NextUntagged: %v", err)
	}
	if post == nil || post.Author != "Bob" {
		t.Fatalf("expected Bob (newer post) first, got %+v", post)
	}

	filename, err := FilenameFor(db, post.HTMLHash)
	if err != nil {
		t.Fatalf("FilenameFor: %v", err)
	}
	if strings.TrimSpace(filename) != "test.html" {
		t.Fatalf("expected filename 'test.html', got %q", filename)
	}

	identity, err := IdentityFor(db, post.HTMLHash)
	if err != nil {
		t.Fatalf("IdentityFor: %v", err)
	}

	next, err := NextUntagged(db, []string{identity})
	if err != nil {
		t.Fatalf("NextUntagged with skip: %v", err)
	}
	if next == nil || next.Author != "Alice" {
		t.Fatalf("expected Bob's post to be skipped in favor of Alice's, got %+v", next)
	}

	// Skipping must not affect the remaining count -- it's a navigation
	// concern, not a classification.
	count, err := CountUntagged(db)
	if err != nil {
		t.Fatalf("CountUntagged: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected skip to leave the untagged count unchanged at 2, got %d", count)
	}
}

// TestSearchAndRetag covers finding a post by text regardless of its tag
// status, and correcting its tag via SetTags -- including propagation across
// a duplicate ingestion, and untagging when nothing is checked.
func TestSearchAndRetag(t *testing.T) {
	db := testDB(t)

	now := time.Now().UTC().Truncate(time.Second)
	insertTestPost(t, db, "hash-dup-a", "Carol", now.Add(-1*time.Hour), "fb-search-1")
	insertTestPost(t, db, "hash-dup-b", "Carol", now, "fb-search-1")
	insertTestPost(t, db, "hash-solo", "Dave", now.Add(-2*time.Hour), "")

	tags, err := AllTags(db)
	if err != nil {
		t.Fatalf("AllTags: %v", err)
	}
	var stolenID, abandonedID int64
	for _, tg := range tags {
		switch tg.Name {
		case "stolen":
			stolenID = tg.ID
		case "abandoned":
			abandonedID = tg.ID
		}
	}

	// Tag the duplicate group as stolen up front, via the normal additive
	// path, so there's something to correct.
	if err := SaveTags(db, "hash-dup-a", []int64{stolenID}); err != nil {
		t.Fatalf("SaveTags: %v", err)
	}

	results, err := SearchPosts(db, "carol")
	if err != nil {
		t.Fatalf("SearchPosts: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for Carol (duplicate group collapsed), got %d", len(results))
	}
	if !results[0].HasTag(stolenID) {
		t.Fatalf("expected the search result to show the current 'stolen' tag, got %+v", results[0].Tags)
	}

	// Retag as abandoned instead -- this must clear "stolen", not just add
	// "abandoned" alongside it, and must propagate to hash-dup-b too.
	if err := SetTags(db, results[0].HTMLHash, []int64{abandonedID}); err != nil {
		t.Fatalf("SetTags: %v", err)
	}

	for _, hash := range []string{"hash-dup-a", "hash-dup-b"} {
		got, err := TagsFor(db, hash)
		if err != nil {
			t.Fatalf("TagsFor(%s): %v", hash, err)
		}
		if len(got) != 1 || got[0].ID != abandonedID {
			t.Fatalf("expected %s to have only 'abandoned', got %+v", hash, got)
		}
	}

	// Untagging (saving with nothing checked) must be accepted, unlike
	// SaveTags' ErrNoTagsSelected -- a search result may need clearing, not
	// just re-tagging.
	if err := SetTags(db, "hash-dup-a", nil); err != nil {
		t.Fatalf("SetTags with no tags: %v", err)
	}
	got, err := TagsFor(db, "hash-dup-b")
	if err != nil {
		t.Fatalf("TagsFor after untagging: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected untagging to propagate to the duplicate too, got %+v", got)
	}

	// Search must also find posts purely by description text -- 2 results,
	// not 3, since hash-dup-a/hash-dup-b still share an identity and
	// collapse into one (same grouping as the Carol search above).
	results, err = SearchPosts(db, "test post")
	if err != nil {
		t.Fatalf("SearchPosts by description: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results (duplicate group collapsed) matching the shared description text, got %d", len(results))
	}
}
