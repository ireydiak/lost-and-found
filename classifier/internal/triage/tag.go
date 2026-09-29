package triage

import (
	"database/sql"
	"errors"
)

// ErrNoTagsSelected is returned by SaveTags when given no tag IDs. Callers
// should redisplay the post with an inline error, not treat this as a
// server error.
var ErrNoTagsSelected = errors.New("at least one tag must be selected")

// SaveTags records the given tag IDs against a post, and against every
// other html_hash that represents the same real-world post (see
// identityExpr in queue.go) -- e.g. the same post re-sourced in an earlier
// or later scrape. Without this, tagging one copy would leave its
// duplicates permanently stuck in the untagged queue, since posts_categories
// is keyed per html_hash (per ingestion), not per real-world post. A tag
// already recorded for a post is silently skipped (ON CONFLICT DO NOTHING),
// making a double-submit harmless.
func SaveTags(db *sql.DB, htmlHash string, tagIDs []int64) error {
	if len(tagIDs) == 0 {
		return ErrNoTagsSelected
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	hashes, err := identityGroup(tx, htmlHash)
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`INSERT INTO posts_categories (tag_id, html_hash) VALUES ($1, $2) ON CONFLICT DO NOTHING`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, hash := range hashes {
		for _, id := range tagIDs {
			if _, err := stmt.Exec(id, hash); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// SetTags replaces htmlHash's entire identity group's tag set (see
// identityExpr) with exactly tagIDs, clearing whatever tags it currently
// has first -- unlike SaveTags (additive, meant for first-time tagging from
// the triage queue), this is for correcting a post found via search: saving
// a different set of boxes actually reclassifies it instead of leaving the
// wrong tag alongside the new one. An empty tagIDs untags the group
// entirely, which is intentional (a search result may need to be
// un-tagged, not just re-tagged).
func SetTags(db *sql.DB, htmlHash string, tagIDs []int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	hashes, err := identityGroup(tx, htmlHash)
	if err != nil {
		return err
	}

	for _, hash := range hashes {
		if _, err := tx.Exec(`DELETE FROM posts_categories WHERE html_hash = $1`, hash); err != nil {
			return err
		}
		for _, id := range tagIDs {
			if _, err := tx.Exec(
				`INSERT INTO posts_categories (tag_id, html_hash) VALUES ($1, $2)`,
				id, hash,
			); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// identityGroup returns every html_hash sharing htmlHash's identity (see
// identityExpr in queue.go) -- always including htmlHash itself, even when
// it has no duplicates.
func identityGroup(tx *sql.Tx, htmlHash string) ([]string, error) {
	rows, err := tx.Query(`
		SELECT pd.html_hash
		FROM posts_details pd
		INNER JOIN posts_meta pm ON pm.html_hash = pd.html_hash
		WHERE `+identityExpr+` = (
			SELECT `+identityExpr+`
			FROM posts_details pd
			INNER JOIN posts_meta pm ON pm.html_hash = pd.html_hash
			WHERE pd.html_hash = $1
		)
	`, htmlHash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hashes []string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return nil, err
		}
		hashes = append(hashes, h)
	}
	return hashes, rows.Err()
}
