// Package fbpost extracts structured post data from saved Facebook group
// feed HTML (Chrome/Chrome-DevTools-sourced — see boundary.go for why post
// boundaries can't be found with a simple selector for this source).
package fbpost

import "time"

type Post struct {
	// Position of this post within its own extraction run (i.e. within one
	// call to ExtractPosts). NOT Facebook's aria-posinset — that attribute
	// doesn't exist in Chrome/Chrome-DevTools-sourced HTML at all. Not a
	// stable ID across files/runs; callers merging posts from multiple
	// files should dedupe on content (Author + Description + Date), not
	// this field.
	PosInset    int       `json:"posInset"`
	Author      string    `json:"author"`
	Description *string   `json:"description"`
	Date        time.Time `json:"date"`
	Picture     *string   `json:"picture"`
}
