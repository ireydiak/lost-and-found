package triage

import (
	"time"

	"domain"
)

// TriagePost is a post as shown in the triage queue: domain.Post plus
// ingestion-layer metadata (filename, sourced_at) useful for debugging
// extraction issues -- e.g. tracing a suspiciously short description back to
// its raw source file. Not part of domain.Post: provenance about how a post
// was sourced isn't part of what a post fundamentally is (the same split
// already applied to dedup_key -- see extractor/cmd/load).
type TriagePost struct {
	domain.Post
	Filename  string
	SourcedAt time.Time
}

// Tag is one of the available classification tags (e.g. "stolen").
type Tag struct {
	ID   int64
	Name string
}

// SearchResult is a post found via SearchPosts, along with the tags it's
// currently assigned -- so the search page can pre-check them for retagging.
type SearchResult struct {
	TriagePost
	Tags []Tag
}

// HasTag reports whether id is among this result's current tags -- used by
// the search template to pre-check the matching checkbox.
func (r SearchResult) HasTag(id int64) bool {
	for _, t := range r.Tags {
		if t.ID == id {
			return true
		}
	}
	return false
}
