package fbpost

const (
	// The post boundary itself (a wrapper element/attribute scoping exactly
	// one post) does not exist in Chrome/Chrome-DevTools-sourced HTML: no
	// aria-posinset, no usable role="article" (matches empty decorative
	// wrappers), no html-div-classed ancestor that cleanly scopes one post.
	// Instead, posts are found by pairing profile_name/story_message markers
	// in document order — see findPosts() in boundary.go.
	profileNameSelector = `[data-ad-rendering-role="profile_name"]`
	authorSelector      = `[data-ad-rendering-role="profile_name"] a`
	// story_message covers a normal post's text; blockquote covers a shared/
	// cross-posted post's embedded content (which has no story_message of
	// its own).
	messageSelector = `[data-ad-rendering-role="story_message"], blockquote`
	// href*= (substring), not href^= (prefix): the timestamp link's href isn't
	// always "?__cft__..." — on some post types (e.g. cross-posts) it's a full
	// permalink URL like "https://www.facebook.com/groups/.../posts/<id>?...__cft__...".
	// findPostDate() filters candidates by content (does the text parse as a
	// date), so casting a wider net here is safe.
	dateSelector = `a[href*="__cft__"]`
	// Most posts nest their text in one or more div[dir="auto"] elements.
	// Some post types (a bold "title" line via <h3><strong>, followed by
	// plain paragraph divs, all wrapped in one outer span[dir="auto"]) have
	// no div[dir="auto"] of their own — descriptionSpanFallbackSelector
	// catches those. See extractDescription() in extract.go for how the two
	// are combined.
	descriptionSelector             = `[data-ad-rendering-role="story_message"] div[dir="auto"]`
	descriptionSpanFallbackSelector = `[data-ad-rendering-role="story_message"] span[dir="auto"]`
	// Facebook's "colorful background" stylized text posts (a big centered
	// heading over a photo background — an opt-in composer style) have no
	// dir="auto" anywhere. Their text lives in a div[style*="text-shadow"],
	// duplicated once inside a decorative aria-hidden="true" copy and once
	// for real — extractDescription() filters out the hidden copy via
	// Selection.Closest, since a CSS selector alone can't express "not
	// inside an aria-hidden ancestor".
	descriptionStyledTextSelector = `[data-ad-rendering-role="story_message"] [style*="text-shadow"]`
	// data-imgperflogname="feedImage" only appears on single-image posts —
	// multi-image (gallery) posts have no such marker on any of their <img>
	// tags. pictureFallbackSelector catches those: scontent.*.fna.fbcdn.net
	// is Facebook's user-content CDN (real uploaded photos), distinct from
	// static.xx.fbcdn.net (avatars, UI icons) and inline data: URIs (SVG
	// icons) — verified against real posts to reliably skip the author's
	// avatar and land on the first actual photo.
	pictureSelector         = `img[data-imgperflogname="feedImage"]`
	pictureFallbackSelector = `img[src^="https://scontent"]`
	// The "colorful background" post style (see descriptionStyledTextSelector
	// above) also has no <img> for its photo at all — it's a CSS
	// background-image on the container div's inline style instead.
	pictureBackgroundImageSelector = `[data-ad-rendering-role="story_message"] [style*="background-image"]`

	// U+034F COMBINING GRAPHEME JOINER — Facebook interleaves this invisible
	// character between the digits/letters of relative timestamps to make
	// naive text scraping harder (e.g. "1h" arrives as "1͏h͏").
	graphemeJoinerUnicode = "͏"

	// U+202F NARROW NO-BREAK SPACE -- Facebook uses this instead of a regular
	// space between the time and AM/PM in absolute dates copied from
	// declarative shadow DOM content (e.g. "11:59 AM" with U+202F, not a
	// normal space). Normalized to a regular space rather than stripped,
	// since time.Parse needs it as a literal separator.
	narrowNoBreakSpaceUnicode = " "

	// Absolute date formats seen behind closed shadow DOM, in increasing
	// post age: a same-day post shows a time ("August 21 at 11:59 AM"), an
	// older-but-same-year post drops the time ("January 4"), and a
	// year-or-more-old post adds an explicit year instead ("December 30,
	// 2025"). Only the first two need year inference (see yearInferer) —
	// the third already states its year.
	absoluteDateWithTimeLayout = "January 2 at 3:04 PM"
	absoluteDateBareLayout     = "January 2"
	absoluteDateWithYearLayout = "January 2, 2006"
)
