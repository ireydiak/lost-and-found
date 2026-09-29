package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"
)

// anonymousAuthor is the fallback label extraction gives a post with no
// linkable profile (see extractor/docs/DATABASE.md, posts_details.author).
// It doesn't identify a real individual, so same-day collisions among
// distinct anonymous posters are expected rather than exceptional.
const anonymousAuthor = "Anonymous participant"

// dedupKey computes posts_details.dedup_key as hash(author, date) --
// deliberately excluding title/description, since those can be edited after
// the fact and the key is meant to recognize a post's original content, not
// track its edit history. See extractor/docs/DATABASE.md, "Deduplication".
//
// For anonymousAuthor, a random salt is mixed in so the key effectively
// never coincides with another anonymous post's key -- these are always
// treated as distinct here, with only facebook_id able to later prove two
// of them are the same real-world post.
func dedupKey(author string, date time.Time) (string, error) {
	h := sha256.New()
	h.Write([]byte(author))
	h.Write([]byte("|"))
	h.Write([]byte(strconv.FormatInt(date.Unix(), 10)))

	if author == anonymousAuthor {
		salt := make([]byte, 16)
		if _, err := rand.Read(salt); err != nil {
			return "", err
		}
		h.Write(salt)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
