package fbpost

import (
	"os"

	"github.com/PuerkitoBio/goquery"
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
func ExtractPostsFromFile(path string) ([]Post, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	doc, err := goquery.NewDocumentFromReader(f)
	if err != nil {
		return nil, err
	}

	return ExtractPosts(doc, info.ModTime()), nil
}
