// Package domain holds the data shape shared between extractor (which
// produces posts) and classifier (which reads and tags them). It contains
// only types -- no extraction logic, no database code, no web code.
package domain

import "time"

// AssetType identifies what kind of media an Asset is.
type AssetType string

const (
	AssetTypePicture AssetType = "picture"
	AssetTypeVideo   AssetType = "video"
)

// Asset is one picture or video attached to a post.
type Asset struct {
	URL  string
	Type AssetType
}

// Post is a single Facebook post, as produced by extraction and consumed by
// anything reading it back out of the database.
type Post struct {
	// SHA-256 of the raw source file's bytes.
	HTMLHash string
	// Facebook's own post/permalink ID, when extractable from the source.
	FacebookID  *string
	Author      string
	Title       *string // always nil for now -- title extraction not yet implemented
	Description *string
	Date        time.Time
	Assets      []Asset
}
