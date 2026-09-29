package fbpost

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"

	"github.com/PuerkitoBio/goquery"

	"domain"
)

// ExtractPostsFromFile opens and parses an HTML file, then extracts its
// posts using the file's own modification time as the reference "now" for
// resolving relative dates ("1h ago", "a day ago", "about an hour ago").
//
// Using the actual process run time instead would silently corrupt every
// relative date by however long the gap is between when the file was saved
// and when extraction happens to run — which can be hours, days, or more if
// extraction isn't run immediately after copying. The file's mtime is set
// once, at save time, so it stays correct regardless of that delay.
//
// The file is read into memory once: its bytes are hashed (SHA-256) to
// produce each returned post's HTMLHash, and the same bytes are parsed by
// goquery — avoiding a second read of the file just for hashing.
func ExtractPostsFromFile(path string) ([]domain.Post, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	sum := sha256.Sum256(data)
	htmlHash := hex.EncodeToString(sum[:])

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	posts := ExtractPosts(doc, info.ModTime())
	for i := range posts {
		posts[i].HTMLHash = htmlHash
	}
	return posts, nil
}
