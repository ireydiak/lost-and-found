package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type Post struct {
	PosInset    int       `json:"posInset"`
	Author      string    `json:"author"`
	Description *string   `json:"description"`
	Date        time.Time `json:"date"`
	Picture     *string   `json:"picture"`
}

const (
	postSelector        = `div[role="feed"] div[aria-posinset]`
	authorSelector      = `[data-ad-rendering-role="profile_name"] a`
	dateSelector        = `a[href^="?__cft__"]`
	descriptionSelector = `[data-ad-rendering-role="story_message"] div[dir="auto"]`
	pictureSelector     = `img[data-imgperflogname="feedImage"]`

	// U+034F COMBINING GRAPHEME JOINER — Facebook interleaves this invisible
	// character between the digits/letters of relative timestamps to make
	// naive text scraping harder (e.g. "1h" arrives as "1͏h͏").
	graphemeJoinerUnicode = "\u034F"
)

var (
	dateRegex = regexp.MustCompile(`^(\d+)\s*([mhdw])$`)
)

func main() {
	pathToFile := "../sourcing/debug-page.html"
	f, err := os.Open(pathToFile)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	doc, err := goquery.NewDocumentFromReader(f)
	if err != nil {
		panic(err)
	}

	postsMap := map[int]Post{}
	doc.Find(postSelector).Each(func(_ int, s *goquery.Selection) {
		posInsetStr, ok := s.Attr("aria-posinset")
		if !ok {
			return
		}
		posInset, err := strconv.Atoi(posInsetStr)
		if err != nil {
			return
		}

		if _, exists := postsMap[posInset]; exists {
			return
		}

		authorSel := s.Find(authorSelector).First()
		dateSel := s.Find(dateSelector).First()

		if authorSel.Length() == 0 || dateSel.Length() == 0 {
			return
		}

		dateTime, err := parseDate(dateSel)
		if err != nil {
			return
		}

		p := Post{
			Author:      authorSel.Text(),
			Date:        *dateTime,
			Description: optionalText(s.Find(descriptionSelector).First()),
			Picture:     optionalText(s.Find(pictureSelector).First()),
			PosInset:    posInset,
		}
		postsMap[posInset] = p
	})

	fmt.Println(len(postsMap))
}

func optionalText(s *goquery.Selection) *string {
	if s.Length() == 0 {
		return nil
	}
	text := strings.TrimSpace(s.Text())
	return &text
}

func parseDate(s *goquery.Selection) (*time.Time, error) {
	if s.Length() == 0 {
		return nil, fmt.Errorf("failed to find date from selector")
	}

	text := strings.ReplaceAll(s.Text(), graphemeJoinerUnicode, "")
	text = strings.TrimSpace(text)

	matches := dateRegex.FindStringSubmatch(text)
	if matches == nil {
		return nil, fmt.Errorf("failed to find date from regex")
	}

	// e.g. ["1h", "1", "h"]
	if len(matches) != 3 {
		return nil, fmt.Errorf("failed to parse date: expected len(matches) == 3; got %d", len(matches))
	}

	nStr, unit := matches[1], matches[2]
	n, err := strconv.Atoi(nStr)
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("unrecognized date unit: %q", unit)
	}

	result := time.Now().Add(-duration)
	return &result, nil
}
