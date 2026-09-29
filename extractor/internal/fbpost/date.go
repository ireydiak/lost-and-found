package fbpost

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Three distinct relative-time shapes, seen in different rendering contexts:
// a compact form in ordinary visible text ("1h", "37m", "2d", "3w"), a
// spelled-out English phrase with a count from closed-shadow-DOM template
// content ("56 minutes ago", "21 hours ago", "2 days ago"), and English's
// natural singular phrasing for a count of exactly one ("about an hour ago",
// "a day ago" — no digit at all, "a"/"an" instead of "1").
var (
	compactRelativeDateRegex    = regexp.MustCompile(`^(\d+)\s*([mhdw])$`)
	spelledOutRelativeDateRegex = regexp.MustCompile(`^(\d+)\s+(minute|hour|day|week)s?\s+ago$`)
	singularRelativeDateRegex   = regexp.MustCompile(`^(?:about\s+)?an?\s+(minute|hour|day|week)\s+ago$`)
)

// yearInferer assigns a year to a sequence of month/day values that have no
// year of their own, on the assumption that they are supplied in feed order
// (newest post first) and are therefore non-increasing when read as a plain
// (month, day) pair -- EXCEPT at a year boundary, where the next post's
// (month, day) will appear to jump forward (e.g. "August 21" followed by
// "September 5" can only mean the September post is from the previous year).
// A repeated month (two different posts both in August of the same year) is
// intentionally NOT treated as a year boundary -- only a forward jump is.
type yearInferer struct {
	year         int
	lastMonthDay int // month*100+day of the last date seen, or -1 if none yet
}

func newYearInferer(startYear int) *yearInferer {
	return &yearInferer{year: startYear, lastMonthDay: -1}
}

func (y *yearInferer) yearFor(month time.Month, day int) int {
	monthDay := int(month)*100 + day
	if y.lastMonthDay != -1 && monthDay > y.lastMonthDay {
		y.year--
	}
	y.lastMonthDay = monthDay
	return y.year
}

// setKnownYear records a date whose year is already known (not inferred),
// e.g. "December 30, 2025". This keeps the (month, day) sequence unbroken
// for any subsequent yearFor() call on a later, yearless date — without
// this, the next inferred date would be compared against a stale
// lastMonthDay from before the known-year date.
func (y *yearInferer) setKnownYear(year int, month time.Month, day int) {
	y.year = year
	y.lastMonthDay = int(month)*100 + day
}

func cleanDateText(raw string) string {
	text := strings.ReplaceAll(raw, graphemeJoinerUnicode, "")
	text = strings.ReplaceAll(text, "-", "")
	// Normalizing the narrow no-break space is only needed for absolute dates
	// ("11:59 AM"), but it's harmless to always do — relative dates ("5h")
	// never contain this character.
	text = strings.ReplaceAll(text, narrowNoBreakSpaceUnicode, " ")
	return strings.TrimSpace(text)
}

// parseRelativeDateText handles both relative formats: compact ("1h", "37m",
// "2d", "3w") and spelled-out ("56 minutes ago", "21 hours ago", "2 days
// ago").
func parseRelativeDateText(text string, now time.Time) (time.Time, error) {
	var nStr, unit string

	if m := compactRelativeDateRegex.FindStringSubmatch(text); m != nil {
		// e.g. ["1h", "1", "h"]
		nStr, unit = m[1], m[2]
	} else if m := spelledOutRelativeDateRegex.FindStringSubmatch(text); m != nil {
		// e.g. ["56 minutes ago", "56", "minute"]
		nStr = m[1]
		unit = map[string]string{"minute": "m", "hour": "h", "day": "d", "week": "w"}[m[2]]
	} else if m := singularRelativeDateRegex.FindStringSubmatch(text); m != nil {
		// e.g. ["about an hour ago", "hour"] -- no digit, always means 1.
		nStr = "1"
		unit = map[string]string{"minute": "m", "hour": "h", "day": "d", "week": "w"}[m[1]]
	} else {
		return time.Time{}, fmt.Errorf("text does not match a relative date: %q", text)
	}

	n, err := strconv.Atoi(nStr)
	if err != nil {
		return time.Time{}, err
	}

	var duration time.Duration
	switch unit {
	case "m":
		duration = time.Duration(n) * time.Minute
	case "h":
		duration = time.Duration(n) * time.Hour
	case "d":
		duration = time.Duration(n) * time.Hour * 24
	case "w":
		duration = time.Duration(n) * time.Hour * 24 * 7
	default:
		return time.Time{}, fmt.Errorf("unrecognized date unit: %q", unit)
	}

	return now.Add(-duration), nil
}

// parseAbsoluteDateText handles all three absolute formats, in order from
// most to least specific — a longer string can never accidentally
// short-match a shorter layout in Go's time.Parse (it requires the entire
// input to be consumed), so trying them in any order is safe; this order
// just means the common case is checked first.
func parseAbsoluteDateText(text string, years *yearInferer) (time.Time, error) {
	if t, err := time.Parse(absoluteDateWithYearLayout, text); err == nil {
		years.setKnownYear(t.Year(), t.Month(), t.Day())
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local), nil
	}
	if t, err := time.Parse(absoluteDateWithTimeLayout, text); err == nil {
		year := years.yearFor(t.Month(), t.Day())
		return time.Date(year, t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, time.Local), nil
	}
	if t, err := time.Parse(absoluteDateBareLayout, text); err == nil {
		year := years.yearFor(t.Month(), t.Day())
		return time.Date(year, t.Month(), t.Day(), 0, 0, 0, 0, time.Local), nil
	}
	return time.Time{}, fmt.Errorf("text does not match any absolute date format: %q", text)
}

// parseDateText tries the relative format first, then the absolute format —
// the two are mutually exclusive (a string can't match both), so trying both
// unconditionally and keeping whichever succeeds is safe.
func parseDateText(text string, now time.Time, years *yearInferer) (time.Time, error) {
	if t, err := parseRelativeDateText(text, now); err == nil {
		return t, nil
	}
	if t, err := parseAbsoluteDateText(text, years); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("text does not match any known date format: %q", text)
}
