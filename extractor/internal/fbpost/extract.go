package fbpost

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// Matches a CSS background-image URL, e.g. background-image: url("https://...")
// — used by Facebook's "colorful background" stylized text posts, which have
// no <img> tag for their photo at all.
var backgroundImageURLRegex = regexp.MustCompile(`background-image:\s*url\(["']?([^"'\)]+)["']?\)`)

// ExtractPosts parses every post out of a single saved feed document. Posts
// are expected to appear in feed order (newest first) — required for the
// absolute-date year-inference heuristic (see yearInferer in date.go) to
// work correctly.
//
// A post missing a required field (author or a parseable date) is skipped
// entirely rather than included with a zero value. Optional fields
// (description, picture) are nil when absent.
func ExtractPosts(doc *goquery.Document, now time.Time) []Post {
	years := newYearInferer(now.Year())

	var posts []Post
	for i, boundary := range findPosts(doc) {
		scope := goquery.NewDocumentFromNode(lowestCommonAncestor(boundary.author, boundary.message)).Selection

		// Usually an <a> (a real profile link). Facebook's "Anonymous
		// participant" posts have no profile to link to, so the name
		// renders as plain text (a <div role="button">) instead — fall
		// back to the profile_name block's own text in that case, rather
		// than dropping what's otherwise a fully valid post.
		profileSel := goquery.NewDocumentFromNode(boundary.author).Selection
		var authorName string
		if link := profileSel.Find("a").First(); link.Length() > 0 {
			authorName = strings.TrimSpace(link.Text())
		} else {
			authorName = strings.TrimSpace(profileSel.Text())
		}
		if authorName == "" {
			continue
		}

		dateTime, err := findPostDate(scope, now, years)
		if err != nil {
			continue
		}

		posts = append(posts, Post{
			Author:      authorName,
			Date:        *dateTime,
			Description: extractDescription(scope),
			Picture:     extractPicture(scope),
			PosInset:    i,
		})
	}

	return posts
}

func findPostDate(post *goquery.Selection, now time.Time, years *yearInferer) (*time.Time, error) {
	candidates := post.Find(dateSelector)
	if candidates.Length() == 0 {
		return nil, fmt.Errorf("no date candidates found")
	}

	var result *time.Time
	candidates.EachWithBreak(func(_ int, candidate *goquery.Selection) bool {
		text := cleanDateText(candidate.Text())
		t, err := parseDateText(text, now, years)
		if err != nil {
			return true // keep looking
		}
		result = &t
		return false // found it, stop iterating
	})

	if result == nil {
		return nil, fmt.Errorf("no candidate among %d matched a valid date format", candidates.Length())
	}
	return result, nil
}

// extractDescription reads a post's text. Most posts nest their text in one
// or more div[dir="auto"] elements (descriptionSelector); if none are found,
// fall back to the post's outer span[dir="auto"] wrapper — used by a
// different post template (a bold "title" line via <h3><strong>, followed
// by one or more plain paragraph divs, all wrapped in one span[dir="auto"])
// that has no div[dir="auto"] of its own. The fallback clones the span and
// strips any role="button" descendant first — Facebook's "See more" expand
// link lives as a sibling inside that same span and would otherwise get
// appended to the text.
func extractDescription(scope *goquery.Selection) *string {
	if primary := scope.Find(descriptionSelector).First(); primary.Length() > 0 {
		return optionalText(primary)
	}

	if fallback := scope.Find(descriptionSpanFallbackSelector).First(); fallback.Length() > 0 {
		clone := fallback.Clone()
		clone.Find(`[role="button"]`).Remove()
		return optionalText(clone)
	}

	// "Colorful background" stylized posts: the same text is duplicated
	// inside a decorative aria-hidden="true" copy (for the visual text-
	// shadow/outline effect) and once for real. Closest() can express "skip
	// if inside an aria-hidden ancestor", which a CSS selector alone can't.
	var styled *goquery.Selection
	scope.Find(descriptionStyledTextSelector).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if s.Closest(`[aria-hidden="true"]`).Length() > 0 {
			return true // inside the decorative copy, keep looking
		}
		styled = s
		return false
	})
	// styled stays a literal Go nil (not an empty-but-valid *Selection) if
	// no candidate survived the filter above — optionalText calls
	// .Length() on it, which would panic on a true nil receiver.
	if styled != nil {
		return optionalText(styled)
	}

	// Last resort: some short/simple posts have plain text with no
	// dir="auto" anywhere, no h3/title structure, and no text-shadow style
	// -- just a bare span>div. Grab the whole story_message's text,
	// stripping button controls ("See more") and any aria-hidden="true"
	// decorative duplicate. Only reached when all three more targeted
	// tiers above already failed, so this can't shadow-duplicate a
	// translated sibling div[dir="auto"] (tier 1 would have already
	// matched and returned for that case).
	// story_message specifically, not messageSelector (which also matches
	// blockquote) -- a cross-post's boilerplate/quoted blockquote content
	// must not be picked up here instead of the real post text.
	message := scope.Find(`[data-ad-rendering-role="story_message"]`).First()
	if message.Length() == 0 {
		return nil
	}
	clone := message.Clone()
	clone.Find(`[role="button"], [aria-hidden="true"]`).Remove()
	return optionalText(clone)
}

// extractPicture reads a post's photo URL. Single-image posts carry
// data-imgperflogname="feedImage" (pictureSelector); multi-image (gallery)
// posts have no such marker on any image, so fall back to the first
// scontent-CDN-hosted image (pictureFallbackSelector) — see the comment on
// that selector for why this reliably skips the author's avatar. "Colorful
// background" stylized posts have no <img> for their photo at all — it's a
// CSS background-image on the container's inline style instead.
func extractPicture(scope *goquery.Selection) *string {
	if primary := scope.Find(pictureSelector).First(); primary.Length() > 0 {
		return optionalAttr(primary, "src")
	}
	if fallback := scope.Find(pictureFallbackSelector).First(); fallback.Length() > 0 {
		return optionalAttr(fallback, "src")
	}

	bgDiv := scope.Find(pictureBackgroundImageSelector).First()
	if bgDiv.Length() == 0 {
		return nil
	}
	style, _ := bgDiv.Attr("style")
	m := backgroundImageURLRegex.FindStringSubmatch(style)
	if m == nil {
		return nil
	}
	url := m[1]
	return &url
}

func optionalText(s *goquery.Selection) *string {
	if s.Length() == 0 {
		return nil
	}
	text := strings.TrimSpace(s.Text())
	return &text
}

// optionalAttr reads an attribute (e.g. an <img>'s "src") rather than an
// element's text — needed for void elements like <img>, which have no text
// content of their own.
func optionalAttr(s *goquery.Selection, attr string) *string {
	if s.Length() == 0 {
		return nil
	}
	val, ok := s.Attr(attr)
	if !ok {
		return nil
	}
	return &val
}
