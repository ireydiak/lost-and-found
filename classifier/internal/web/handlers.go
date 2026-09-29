package web

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime/debug"
	"strconv"
	"sync"

	"classifier/internal/triage"
)

//go:embed templates/*.html
var templatesFS embed.FS

// Server holds the dependencies HTTP handlers need.
type Server struct {
	db    *sql.DB
	tmpl  *template.Template
	isDev bool

	skipMu  sync.Mutex
	skipped []string // post identities skipped this run -- see triage.IdentityFor
}

// NewServer builds a Server. isDev controls whether error responses include
// the underlying error and a stack trace (development) or a generic message
// (anything else) -- see APP_ENV in cmd/classifier/main.go.
func NewServer(db *sql.DB, isDev bool) (*Server, error) {
	tmpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Server{db: db, tmpl: tmpl, isDev: isDev}, nil
}

// Routes returns the server's HTTP handler.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleIndex)
	mux.HandleFunc("POST /tag", s.handleTag)
	mux.HandleFunc("POST /skip", s.handleSkip)
	mux.HandleFunc("GET /raw", s.handleRaw)
	mux.HandleFunc("GET /inspect", s.handleInspect)
	mux.HandleFunc("GET /search", s.handleSearch)
	mux.HandleFunc("POST /retag", s.handleRetag)
	return mux
}

type searchPageData struct {
	Query   string
	Results []triage.SearchResult
	Tags    []triage.Tag
}

type pageData struct {
	Post      *triage.TriagePost
	Tags      []triage.Tag
	Remaining int
	Error     string
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	s.renderIndex(w, "")
}

// skippedIdentities returns a snapshot of the skip set, safe to hand to a
// query -- skipMu only needs to protect the slice itself, not the query.
func (s *Server) skippedIdentities() []string {
	s.skipMu.Lock()
	defer s.skipMu.Unlock()
	return append([]string(nil), s.skipped...)
}

func (s *Server) renderIndex(w http.ResponseWriter, errMsg string) {
	post, err := triage.NextUntagged(s.db, s.skippedIdentities())
	if err != nil {
		s.serverError(w, err)
		return
	}

	tags, err := triage.AllTags(s.db)
	if err != nil {
		s.serverError(w, err)
		return
	}

	remaining, err := triage.CountUntagged(s.db)
	if err != nil {
		s.serverError(w, err)
		return
	}

	data := pageData{Post: post, Tags: tags, Remaining: remaining, Error: errMsg}
	if err := s.tmpl.ExecuteTemplate(w, "triage.html", data); err != nil {
		s.serverError(w, err)
	}
}

func (s *Server) handleTag(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.serverError(w, err)
		return
	}

	htmlHash := r.FormValue("html_hash")

	var tagIDs []int64
	for _, raw := range r.Form["tag_id"] {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			s.serverError(w, err)
			return
		}
		tagIDs = append(tagIDs, id)
	}

	if err := triage.SaveTags(s.db, htmlHash, tagIDs); err != nil {
		if errors.Is(err, triage.ErrNoTagsSelected) {
			s.renderIndex(w, "Please select at least one tag.")
			return
		}
		s.serverError(w, err)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleSkip records the current post as not ready to tag yet, so it's
// passed over for the rest of this run -- it isn't tagged and isn't removed
// from "remaining", it just stops showing up as the *next* post. Restarting
// the server (e.g. `make classify`) clears the skip set.
func (s *Server) handleSkip(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.serverError(w, err)
		return
	}

	htmlHash := r.FormValue("html_hash")
	identity, err := triage.IdentityFor(s.db, htmlHash)
	if err != nil {
		s.serverError(w, err)
		return
	}

	s.skipMu.Lock()
	s.skipped = append(s.skipped, identity)
	s.skipMu.Unlock()

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleRaw serves a post's raw source HTML verbatim as text/plain -- a
// browser never executes text/plain content regardless of what's in it, and
// it's directly usable via curl or fetch() (handleInspect's page fetches
// this to build its pretty-printed/queryable view). The file path is looked
// up from posts_meta by html_hash rather than accepted directly, so this
// can only ever serve a file this tool itself recorded as sourced.
func (s *Server) handleRaw(w http.ResponseWriter, r *http.Request) {
	htmlHash := r.URL.Query().Get("html_hash")

	filename, err := triage.FilenameFor(s.db, htmlHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		s.serverError(w, err)
		return
	}

	content, err := os.ReadFile(filename)
	if err != nil {
		s.serverError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(content)
}

type inspectPageData struct {
	HTMLHash string
}

// handleInspect renders an interactive debugging page for a post's raw
// source: it fetches handleRaw's plain-text content client-side, parses it
// with DOMParser into an inert document (embedded <script> tags in the
// saved markup never run -- DOMParser-parsed documents have no browsing
// context), pretty-prints it, and lets a JS expression be run against it
// (e.g. doc.querySelectorAll('div[dir="auto"]')) -- the same query language
// the extractor's own selectors use, so a selector can be tried against the
// real saved markup directly.
func (s *Server) handleInspect(w http.ResponseWriter, r *http.Request) {
	htmlHash := r.URL.Query().Get("html_hash")

	// Confirmed to exist before rendering, so a bad link 404s instead of
	// showing a page whose fetch() silently fails.
	if _, err := triage.FilenameFor(s.db, htmlHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		s.serverError(w, err)
		return
	}

	data := inspectPageData{HTMLHash: htmlHash}
	if err := s.tmpl.ExecuteTemplate(w, "inspect.html", data); err != nil {
		s.serverError(w, err)
	}
}

// handleSearch looks up posts by author or description text, regardless of
// tag status, so an already-tagged post can be found and corrected -- see
// handleRetag. An empty query just shows the search form with no results.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")

	var results []triage.SearchResult
	if query != "" {
		var err error
		results, err = triage.SearchPosts(s.db, query)
		if err != nil {
			s.serverError(w, err)
			return
		}
	}

	tags, err := triage.AllTags(s.db)
	if err != nil {
		s.serverError(w, err)
		return
	}

	data := searchPageData{Query: query, Results: results, Tags: tags}
	if err := s.tmpl.ExecuteTemplate(w, "search.html", data); err != nil {
		s.serverError(w, err)
	}
}

// handleRetag replaces a post's tag set with whatever's checked on the
// search page -- see triage.SetTags. Unlike handleTag, submitting with
// nothing checked is valid (it untags the post) rather than an error, since
// correcting a wrong tag can legitimately mean "this shouldn't be tagged at
// all".
func (s *Server) handleRetag(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.serverError(w, err)
		return
	}

	htmlHash := r.FormValue("html_hash")
	query := r.FormValue("q")

	var tagIDs []int64
	for _, raw := range r.Form["tag_id"] {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			s.serverError(w, err)
			return
		}
		tagIDs = append(tagIDs, id)
	}

	if err := triage.SetTags(s.db, htmlHash, tagIDs); err != nil {
		s.serverError(w, err)
		return
	}

	http.Redirect(w, r, "/search?q="+url.QueryEscape(query), http.StatusSeeOther)
}

func (s *Server) serverError(w http.ResponseWriter, err error) {
	log.Printf("error: %v", err)
	w.WriteHeader(http.StatusInternalServerError)
	if s.isDev {
		fmt.Fprintf(w, "<pre>%s\n\n%s</pre>", err.Error(), debug.Stack())
		return
	}
	fmt.Fprint(w, "<h1>Something went wrong</h1>")
}
