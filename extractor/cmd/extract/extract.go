package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"domain"
	"extractor/internal/fbpost"
)

func main() {
	watchDir := flag.String("watch", "", "watch this directory for new .html files and extract each one as it appears, instead of extracting a single file")
	flag.Parse()

	if *watchDir != "" {
		watch(*watchDir)
		return
	}

	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage:\n  %s <path-to-html-file>\n  %s <directory-of-html-files>\n  %s --watch <directory>\n", os.Args[0], os.Args[0], os.Args[0])
		os.Exit(1)
	}

	path := args[0]
	info, err := os.Stat(path)
	if err != nil {
		panic(err)
	}

	if info.IsDir() {
		printJSON(extractDir(path))
		return
	}

	posts, err := fbpost.ExtractPostsFromFile(path)
	if err != nil {
		panic(err)
	}
	printJSON(posts)
}

// extractDir extracts posts from every .html file directly inside dir
// (non-recursive), skipping files that fail to extract.
func extractDir(dir string) []domain.Post {
	files, err := filepath.Glob(filepath.Join(dir, "*.html"))
	if err != nil {
		panic(err)
	}
	sort.Strings(files)

	var all []domain.Post
	for _, f := range files {
		posts, err := fbpost.ExtractPostsFromFile(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skipping %s: %v\n", f, err)
			continue
		}
		all = append(all, posts...)
	}
	return all
}

func printJSON(posts []domain.Post) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(posts); err != nil {
		panic(err)
	}
}

// watch polls dir for new .html files and extracts each one as it appears —
// the automatic counterpart to manually running this program once per file.
// Files already present when watching starts are NOT reprocessed; run this
// program's single-file mode or `merge` on the directory first to catch up
// on anything saved before the watcher was started.
//
// A newly-created file is only processed once its size has been observed as
// stable across two consecutive polls, to avoid reading it mid-write (e.g.
// while save-post.sh's shell redirect is still in progress).
func watch(dir string) {
	const pollInterval = 2 * time.Second

	seen := map[string]bool{}
	pendingSize := map[string]int64{}

	initial, err := os.ReadDir(dir)
	if err != nil {
		panic(err)
	}
	for _, e := range initial {
		seen[e.Name()] = true
	}

	fmt.Fprintf(os.Stderr, "Watching %s for new .html files (Ctrl+C to stop)...\n", dir)

	for {
		entries, err := os.ReadDir(dir)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error reading directory:", err)
			time.Sleep(pollInterval)
			continue
		}

		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || seen[name] || !strings.HasSuffix(name, ".html") {
				continue
			}

			info, err := e.Info()
			if err != nil {
				continue
			}
			size := info.Size()

			if prevSize, ok := pendingSize[name]; ok && prevSize == size {
				delete(pendingSize, name)
				seen[name] = true
				processNewFile(filepath.Join(dir, name))
			} else {
				pendingSize[name] = size
			}
		}

		time.Sleep(pollInterval)
	}
}

func processNewFile(path string) {
	posts, err := fbpost.ExtractPostsFromFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[%s] skipping %s: %v\n", time.Now().Format("15:04:05"), filepath.Base(path), err)
		return
	}
	fmt.Fprintf(os.Stderr, "[%s] extracted %d post(s) from %s\n", time.Now().Format("15:04:05"), len(posts), filepath.Base(path))
	printJSON(posts)
}
