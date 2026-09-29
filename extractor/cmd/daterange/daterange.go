package main

import (
	"fmt"
	"os"
	"path/filepath"

	"domain"
	"extractor/internal/fbpost"
)

// daterange reports the oldest and newest post dates found across a
// directory of saved HTML snapshots — useful when starting a new manual
// copy-paste session, to see where a previous session left off.
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

	var oldest, newest *domain.Post
	total := 0

	for _, path := range files {
		// Each file's own modification time is used as "now" for resolving
		// its relative dates -- see ExtractPostsFromFile.
		posts, err := fbpost.ExtractPostsFromFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skipping %s: %v\n", path, err)
			continue
		}

		for _, p := range posts {
			total++
			p := p
			if oldest == nil || p.Date.Before(oldest.Date) {
				oldest = &p
			}
			if newest == nil || p.Date.After(newest.Date) {
				newest = &p
			}
		}
	}

	if total == 0 {
		fmt.Println("No posts extracted from", len(files), "file(s).")
		return
	}

	fmt.Printf("Scanned %d file(s), %d post(s) extracted\n\n", len(files), total)
	fmt.Printf("Oldest: %s — %s (%s)\n", oldest.Date.Format("2006-01-02"), oldest.Author, oldest.Date.Format("15:04"))
	fmt.Printf("Newest: %s — %s (%s)\n", newest.Date.Format("2006-01-02"), newest.Author, newest.Date.Format("15:04"))
}
