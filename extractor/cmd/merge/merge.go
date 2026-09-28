package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"extractor/internal/fbpost"
)

// dedupeKey is a post's content-based identity, used to merge the same
// real-world post seen across multiple HTML snapshots (manual copies at
// different scroll depths, or repeated scrapes) into one entry.
//
// Picture is deliberately excluded: Facebook's CDN URLs embed per-request
// signing tokens (_nc_gid, oh=, etc.) that differ between requests even for
// the exact same image, so including it would treat the same real-world
// post as "different" every time it's re-scraped.
type dedupeKey struct {
	author      string
	description string
	dateUnix    int64
}

func dedupeKeyFor(p fbpost.Post) dedupeKey {
	desc := ""
	if p.Description != nil {
		desc = *p.Description
	}
	return dedupeKey{author: p.Author, description: desc, dateUnix: p.Date.Unix()}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <directory-of-html-files>\n", os.Args[0])
		os.Exit(1)
	}
	dir := os.Args[1]

	files, err := filepath.Glob(filepath.Join(dir, "*.html"))
	if err != nil {
		panic(err)
	}
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "no .html files found in %s\n", dir)
		os.Exit(1)
	}
	sort.Strings(files)

	seen := map[dedupeKey]fbpost.Post{}

	for _, path := range files {
		// Each file's own modification time is used as "now" for resolving
		// its relative dates ("1h ago") -- not a single shared time.Now()
		// for the whole batch, since files in this directory can span many
		// separate copy sessions saved days or weeks apart.
		posts, err := fbpost.ExtractPostsFromFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skipping %s: %v\n", path, err)
			continue
		}

		for _, p := range posts {
			key := dedupeKeyFor(p)
			if _, exists := seen[key]; !exists {
				seen[key] = p
			}
		}
	}

	merged := make([]fbpost.Post, 0, len(seen))
	for _, p := range seen {
		merged = append(merged, p)
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].Date.After(merged[j].Date) })

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(merged); err != nil {
		panic(err)
	}
}
