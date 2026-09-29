package triage

import (
	"database/sql"

	"github.com/lib/pq"

	"domain"
)

// identityExpr groups posts_details/posts_meta rows that represent the same
// real-world post -- matched by facebook_id when present, falling back to
// dedup_key otherwise (see extractor/docs/DATABASE.md, "Deduplication"). The
// same post re-sourced across multiple scrape sessions legitimately produces
// one posts_meta/posts_details row per ingestion; without this grouping,
// every re-scraped copy would surface as its own separate untagged post.
const identityExpr = `COALESCE(pm.facebook_id, pd.dedup_key)`

// NextUntagged returns the next post with no tags assigned yet, or nil (with
// no error) if none remain. One row per real-world post is considered, even
// if it was sourced multiple times -- see identityExpr.
//
// skipIdentities excludes posts whose identity (see identityExpr) is in the
// list -- used to let a user move past a post that isn't ready to be tagged
// yet without actually tagging it. Pass nil to consider every untagged post.
func NextUntagged(db *sql.DB, skipIdentities []string) (*TriagePost, error) {
	// pq.Array(nil) encodes to SQL NULL, not an empty array -- and
	// `x = ANY(NULL)` is NULL, which a WHERE clause treats as false,
	// silently excluding every row. A non-nil empty slice encodes to '{}',
	// against which `x = ANY('{}')` is false for every row as intended.
	if skipIdentities == nil {
		skipIdentities = []string{}
	}

	row := db.QueryRow(`
		SELECT html_hash, author, title, description, date, filename, sourced_at FROM (
			SELECT DISTINCT ON (`+identityExpr+`)
				pd.html_hash, pd.author, pd.title, pd.description, pd.date, pm.filename, pm.sourced_at
			FROM posts_details pd
			INNER JOIN posts_meta pm ON pm.html_hash = pd.html_hash
			LEFT JOIN posts_categories pc ON pc.html_hash = pd.html_hash
			WHERE pc.html_hash IS NULL
			  AND NOT (`+identityExpr+` = ANY($1))
			ORDER BY `+identityExpr+`, pd.date DESC
		) grouped
		ORDER BY date DESC
		LIMIT 1
	`, pq.Array(skipIdentities))

	var p TriagePost
	if err := row.Scan(&p.HTMLHash, &p.Author, &p.Title, &p.Description, &p.Date, &p.Filename, &p.SourcedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	assets, err := assetsFor(db, p.HTMLHash)
	if err != nil {
		return nil, err
	}
	p.Assets = assets

	return &p, nil
}

// IdentityFor returns htmlHash's identity (see identityExpr) -- used to
// resolve which real-world post to add to the skip set when the user skips
// one of its ingestions.
func IdentityFor(db *sql.DB, htmlHash string) (string, error) {
	var identity string
	err := db.QueryRow(`
		SELECT `+identityExpr+`
		FROM posts_details pd
		INNER JOIN posts_meta pm ON pm.html_hash = pd.html_hash
		WHERE pd.html_hash = $1
	`, htmlHash).Scan(&identity)
	return identity, err
}

// FilenameFor returns the source file path recorded for htmlHash, for
// serving its raw HTML back for debugging.
func FilenameFor(db *sql.DB, htmlHash string) (string, error) {
	var filename string
	err := db.QueryRow(`SELECT filename FROM posts_meta WHERE html_hash = $1`, htmlHash).Scan(&filename)
	return filename, err
}

func assetsFor(db *sql.DB, htmlHash string) ([]domain.Asset, error) {
	rows, err := db.Query(`SELECT url, asset_type FROM posts_assets WHERE html_hash = $1`, htmlHash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assets []domain.Asset
	for rows.Next() {
		var a domain.Asset
		if err := rows.Scan(&a.URL, &a.Type); err != nil {
			return nil, err
		}
		assets = append(assets, a)
	}
	return assets, rows.Err()
}

// CountUntagged returns how many real-world posts have no tags assigned
// yet -- counting each identity group (see identityExpr) once, regardless
// of how many times it was sourced.
func CountUntagged(db *sql.DB) (int, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(DISTINCT `+identityExpr+`)
		FROM posts_details pd
		INNER JOIN posts_meta pm ON pm.html_hash = pd.html_hash
		LEFT JOIN posts_categories pc ON pc.html_hash = pd.html_hash
		WHERE pc.html_hash IS NULL
	`).Scan(&count)
	return count, err
}

// SearchPosts returns up to 50 posts whose author or description contains
// query (case-insensitive), one per real-world post (see identityExpr),
// most recent first -- for finding a specific post to retag, regardless of
// its current tag status. Each result carries its currently-assigned tags
// (see TagsFor) so the search page can pre-check them.
func SearchPosts(db *sql.DB, query string) ([]SearchResult, error) {
	like := "%" + query + "%"
	rows, err := db.Query(`
		SELECT html_hash, author, title, description, date, filename, sourced_at FROM (
			SELECT DISTINCT ON (`+identityExpr+`)
				pd.html_hash, pd.author, pd.title, pd.description, pd.date, pm.filename, pm.sourced_at
			FROM posts_details pd
			INNER JOIN posts_meta pm ON pm.html_hash = pd.html_hash
			WHERE pd.description ILIKE $1 OR pd.author ILIKE $1
			ORDER BY `+identityExpr+`, pd.date DESC
		) grouped
		ORDER BY date DESC
		LIMIT 50
	`, like)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.HTMLHash, &r.Author, &r.Title, &r.Description, &r.Date, &r.Filename, &r.SourcedAt); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range results {
		assets, err := assetsFor(db, results[i].HTMLHash)
		if err != nil {
			return nil, err
		}
		results[i].Assets = assets

		tags, err := TagsFor(db, results[i].HTMLHash)
		if err != nil {
			return nil, err
		}
		results[i].Tags = tags
	}

	return results, nil
}

// TagsFor returns the tags currently assigned to htmlHash.
func TagsFor(db *sql.DB, htmlHash string) ([]Tag, error) {
	rows, err := db.Query(`
		SELECT t.tag_id, t.name
		FROM posts_categories pc
		INNER JOIN tags t ON t.tag_id = pc.tag_id
		WHERE pc.html_hash = $1
		ORDER BY t.tag_id
	`, htmlHash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// AllTags returns every available tag, for rendering the checkbox list.
// Fetched dynamically rather than hardcoded so a future tag added via a new
// seed shows up without a code change.
func AllTags(db *sql.DB) ([]Tag, error) {
	rows, err := db.Query(`SELECT tag_id, name FROM tags ORDER BY tag_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}
