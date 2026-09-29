package fbpost

import (
	"regexp"

	"github.com/PuerkitoBio/goquery"
)

// Facebook's own post ID shows up in two different href shapes depending on
// post type: an absolute permalink (/posts/<id>) or a photo-attachment
// reference (fbid=<id>) on a shared photo's link. Measured against a real
// 715-file corpus: /posts/ alone covers 66%, fbid= alone covers 87%, the
// union covers 96%. Neither alone is universal, so both are tried, /posts/
// preferred as the more official post-level identifier.
var (
	postIDRegex = regexp.MustCompile(`/posts/(\d+)`)
	fbidRegex   = regexp.MustCompile(`[?&]fbid=(\d+)`)
)

func extractFacebookID(scope *goquery.Selection) *string {
	if id := firstHrefMatch(scope, postIDRegex); id != nil {
		return id
	}
	return firstHrefMatch(scope, fbidRegex)
}

func firstHrefMatch(scope *goquery.Selection, re *regexp.Regexp) *string {
	var found *string
	scope.Find("a[href]").EachWithBreak(func(_ int, a *goquery.Selection) bool {
		href, ok := a.Attr("href")
		if !ok {
			return true
		}
		if m := re.FindStringSubmatch(href); m != nil {
			id := m[1]
			found = &id
			return false
		}
		return true
	})
	return found
}
