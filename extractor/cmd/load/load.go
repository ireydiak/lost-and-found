package main

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	"domain"
	"extractor/internal/fbpost"
)

// loadResult reports what happened when loading a single file, for the
// caller to fold into the run-wide summary.
type loadResult struct {
	postsLoaded     int
	discardedExtras bool // file had more than one post; only the first was kept
}

// loadFile extracts the first post from path and upserts it into
// posts_meta, posts_details, and posts_assets, keyed on the same html_hash
// that identifies this exact ingestion. posts_details' description and
// title are refreshed on conflict (not just inserted once) so that
// re-running load after an extraction bugfix picks up the improved text for
// files already loaded -- everything else (posts_meta, assets, and
// posts_details' author/date/dedup_key/processed_at) stays a pure
// ON CONFLICT DO NOTHING no-op, since those aren't affected by extraction
// quality changes and dedup_key must stay stable once assigned.
func loadFile(db *sql.DB, path string) (loadResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return loadResult{}, err
	}

	posts, err := fbpost.ExtractPostsFromFile(path)
	if err != nil {
		return loadResult{}, err
	}
	if len(posts) == 0 {
		return loadResult{}, nil
	}

	post := posts[0]
	result := loadResult{postsLoaded: 1, discardedExtras: len(posts) > 1}

	if err := upsertPost(db, path, info.ModTime(), post); err != nil {
		return loadResult{}, err
	}
	return result, nil
}

func upsertPost(db *sql.DB, filename string, sourcedAt time.Time, post domain.Post) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`INSERT INTO posts_meta (html_hash, filename, sourced_at, facebook_id)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (html_hash) DO NOTHING`,
		post.HTMLHash, filename, sourcedAt, post.FacebookID,
	); err != nil {
		return fmt.Errorf("posts_meta: %w", err)
	}

	key, err := dedupKey(post.Author, post.Date)
	if err != nil {
		return fmt.Errorf("dedup key: %w", err)
	}

	if _, err := tx.Exec(
		`INSERT INTO posts_details (html_hash, author, title, description, date, dedup_key, processed_at)
		 VALUES ($1, $2, $3, $4, $5, $6, NOW())
		 ON CONFLICT (html_hash) DO UPDATE SET
			 description = EXCLUDED.description,
			 title = EXCLUDED.title`,
		post.HTMLHash, post.Author, post.Title, post.Description, post.Date, key,
	); err != nil {
		return fmt.Errorf("posts_details: %w", err)
	}

	for _, asset := range post.Assets {
		if _, err := tx.Exec(
			`INSERT INTO posts_assets (html_hash, url, asset_type)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (html_hash, url) DO NOTHING`,
			post.HTMLHash, asset.URL, string(asset.Type),
		); err != nil {
			return fmt.Errorf("posts_assets: %w", err)
		}
	}

	return tx.Commit()
}
