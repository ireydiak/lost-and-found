# extractor

Parses saved Facebook group feed HTML snapshots (produced by the `sourcing` scraper) into structured post records.

## Commands

| Command | Description |
|---|---|
| `make build` | Builds the `extract` binary from `./cmd/extract` into `./bin/extract`. |
| `make run` | Runs the previously built `./bin/extract` binary. |

## Facebook feed selectors

These target the DOM structure of a Facebook group's feed page, validated against real scraped HTML. Facebook's own CSS classes (e.g. `x1a2a7pz`) are auto-generated atomic CSS and are not stable — the selectors below rely on semantic attributes instead (`role`, `aria-*`, and `data-ad-rendering-role`, a leftover from Meta's internal ad-rendering pipeline that's also present on organic posts).

| Field | Selector | Required | Notes |
|---|---|---|---|
| Post boundary | `div[role="feed"] div[aria-posinset]` | — | `aria-posinset` is a stable logical index into the feed. Safe to dedupe on across multiple HTML snapshots, unlike raw DOM position, since Facebook's feed is virtualized (off-screen posts can be unmounted and the same post can reappear across snapshots). |
| Author | `[data-ad-rendering-role="profile_name"] a` (take the **first** match) | Yes | Cross-posted/shared posts have two `profile_name` blocks — whoever shared it into this group, and the original poster of the embedded content. The first match in document order is whoever shared it into this group. |
| Date | `a[href^="?__cft__"]` (take the **first** match) | Yes | Also has multiple matches on cross-posts and posts with a shared-link preview; the first is the timestamp. The text is obfuscated with the invisible Unicode character U+034F (COMBINING GRAPHEME JOINER) interleaved between characters (e.g. `"1h"` arrives as `"1͏h͏"`), and depending on how the text is extracted, CSS-hidden decoy characters may be mixed in too — strip both before parsing. |
| Description | `[data-ad-rendering-role="story_message"] div[dir="auto"]` (take the **first** match) | No | The container also holds Facebook's auto-translation as a sibling `div[dir="auto"]`. Taking the first match picks the original-language text over the translation. |
| Picture | `img[data-imgperflogname="feedImage"]` | No | Read the `src` attribute — `<img>` is a void element with no text/inner content. |

Required fields (author, date): if missing, the post should be skipped entirely. Optional fields (description, picture): default to `null`/empty rather than blocking or erroring.
