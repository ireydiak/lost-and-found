package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	_ "github.com/lib/pq"
)

// summary tallies what happened across every file this run touched, printed
// once at the end so a batch load over hundreds of files stays legible.
type summary struct {
	filesProcessed  int
	postsLoaded     int
	filesSkipped    int // extraction found zero posts, or the file failed to parse
	discardedExtras int // file had more than one post; only the first was kept
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage:\n  %s <path-to-html-file>\n  %s <directory-of-html-files>\n", os.Args[0], os.Args[0])
		os.Exit(1)
	}
	path := os.Args[1]

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is not set")
		os.Exit(1)
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to database: %v\n", err)
		os.Exit(1)
	}

	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var files []string
	if info.IsDir() {
		files, err = filepath.Glob(filepath.Join(path, "*.html"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		sort.Strings(files)
	} else {
		files = []string{path}
	}

	var s summary
	for _, f := range files {
		s.filesProcessed++

		result, err := loadFile(db, f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skipping %s: %v\n", f, err)
			s.filesSkipped++
			continue
		}
		if result.postsLoaded == 0 {
			fmt.Fprintf(os.Stderr, "skipping %s: no posts extracted\n", f)
			s.filesSkipped++
			continue
		}

		s.postsLoaded += result.postsLoaded
		if result.discardedExtras {
			fmt.Fprintf(os.Stderr, "warning: %s yielded more than one post; kept only the first\n", f)
			s.discardedExtras++
		}
	}

	fmt.Printf("\nfiles processed:   %d\n", s.filesProcessed)
	fmt.Printf("posts loaded:      %d\n", s.postsLoaded)
	fmt.Printf("files skipped:     %d\n", s.filesSkipped)
	fmt.Printf("discarded extras:  %d\n", s.discardedExtras)
}
