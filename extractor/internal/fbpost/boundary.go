package fbpost

import (
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// ancestorChain returns n and all of its ancestors, root-first.
func ancestorChain(n *html.Node) []*html.Node {
	var chain []*html.Node
	for cur := n; cur != nil; cur = cur.Parent {
		chain = append([]*html.Node{cur}, chain...)
	}
	return chain
}

// lowestCommonAncestor returns the deepest node that is an ancestor of both
// a and b (including a or b themselves, if one contains the other).
func lowestCommonAncestor(a, b *html.Node) *html.Node {
	ca, cb := ancestorChain(a), ancestorChain(b)
	var result *html.Node
	for i := 0; i < len(ca) && i < len(cb); i++ {
		if ca[i] != cb[i] {
			break
		}
		result = ca[i]
	}
	return result
}

// postBoundary is one post's (author, message) marker pair, as found by
// findPosts. The post's overall scope for further extraction (date,
// picture) is the lowest common ancestor of the two.
type postBoundary struct {
	author  *html.Node
	message *html.Node
}

// findPosts locates post boundaries in HTML that has no dedicated post
// wrapper (Chrome/Chrome-DevTools-sourced HTML — see the comment on
// profileNameSelector in selectors.go). It walks the document once,
// collecting profile_name and story_message/blockquote markers in document
// order, then groups each run of consecutive profile_name markers (1 for a
// normal post, 2 for a cross-post: whoever shared it into this group, then
// the original poster of the embedded content) with the message marker that
// follows the run. The run's FIRST profile_name is used as the post's
// author — matching the project's convention of crediting whoever shared it
// into this group, not the original poster, for cross-posts.
func findPosts(doc *goquery.Document) []postBoundary {
	type marker struct {
		node   *html.Node
		isName bool
	}

	var markers []marker
	doc.Find(profileNameSelector + ", " + messageSelector).Each(func(_ int, s *goquery.Selection) {
		role, _ := s.Attr("data-ad-rendering-role")
		markers = append(markers, marker{node: s.Get(0), isName: role == "profile_name"})
	})

	var posts []postBoundary
	var runStart *html.Node
	for _, m := range markers {
		if m.isName {
			if runStart == nil {
				runStart = m.node
			}
		} else if runStart != nil {
			posts = append(posts, postBoundary{author: runStart, message: m.node})
			runStart = nil
		}
	}
	return posts
}
