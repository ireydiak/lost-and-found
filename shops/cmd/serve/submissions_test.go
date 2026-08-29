package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/ireydiak/shops/internal/auth"
	"github.com/ireydiak/shops/internal/nominatim"
)

// TestSubmissionFlowIntegration walks submit → 403 for non-admin review →
// approve → shop created, against the real local database. Skipped unless
// DATABASE_URL is set.
func TestSubmissionFlowIntegration(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	// Registered before the data cleanups so it runs after them (LIFO); a
	// defer would close the connection before t.Cleanup callbacks fire.
	t.Cleanup(func() { db.Close() })
	if err := db.Ping(); err != nil {
		t.Skipf("database not reachable: %v", err)
	}

	authSvc, err := auth.NewService(auth.Config{
		Secret: "integration-test", BaseURL: "http://localhost:8080",
		Admins: []string{"admin@test.local"},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := &server{db: db, geo: &nominatim.Geocoder{}} // geocoder unused: payloads carry explicit locations

	mux := http.NewServeMux()
	mux.Handle("POST /api/submissions", authSvc.RequireUser(http.HandlerFunc(s.createSubmission)))
	mux.Handle("GET /api/submissions", authSvc.RequireAdmin(http.HandlerFunc(s.listSubmissions)))
	mux.Handle("POST /api/submissions/{id}/approve", authSvc.RequireAdmin(s.reviewSubmission(true)))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	userCookie, _ := authSvc.MintSession("rider@test.local")
	adminCookie, _ := authSvc.MintSession("admin@test.local")

	do := func(method, path string, cookie *http.Cookie, body string) (*http.Response, []byte) {
		t.Helper()
		req, err := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if cookie != nil {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		out, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return resp, out
	}

	// anonymous cannot submit
	resp, _ := do("POST", "/api/submissions", nil, `{}`)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous submit = %d, want 401", resp.StatusCode)
	}

	// a logged-in user proposes a new shop with an explicit location
	body := `{
		"name": "Integration Vélo",
		"website": "https://integration.example",
		"instagram_url": "https://instagram.com/integrationvelo",
		"address": {"street_number": "1", "street_name": "rue Test", "city": "Montréal", "postal_code": "H0H 0H0"},
		"location": {"lat": 45.51, "lon": -73.57}
	}`
	resp, out := do("POST", "/api/submissions", userCookie, body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("submit = %d: %s", resp.StatusCode, out)
	}
	var sub struct {
		ID          int64  `json:"id"`
		Status      string `json:"status"`
		SubmittedBy string `json:"submitted_by"`
	}
	if err := json.Unmarshal(out, &sub); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, q := range []string{
			`DELETE FROM shops WHERE name = 'Integration Vélo'`,
			`DELETE FROM submissions WHERE submitted_by LIKE '%@test.local'`,
		} {
			if _, err := db.Exec(q); err != nil {
				t.Errorf("cleanup %q: %v", q, err)
			}
		}
	})
	if sub.Status != "pending" || sub.SubmittedBy != "rider@test.local" {
		t.Fatalf("submission = %+v", sub)
	}

	// the submitter cannot approve their own proposal
	resp, _ = do("POST", fmt.Sprintf("/api/submissions/%d/approve", sub.ID), userCookie, "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("user approve = %d, want 403", resp.StatusCode)
	}

	// the admin sees it pending and approves it
	resp, out = do("GET", "/api/submissions", adminCookie, "")
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(out), "Integration Vélo") {
		t.Fatalf("pending list = %d: %s", resp.StatusCode, out)
	}
	resp, out = do("POST", fmt.Sprintf("/api/submissions/%d/approve", sub.ID), adminCookie, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("approve = %d: %s", resp.StatusCode, out)
	}

	// the shop now exists with the proposed location
	var count int
	err = db.QueryRow(
		`SELECT count(*) FROM shops WHERE name = 'Integration Vélo' AND location IS NOT NULL`).Scan(&count)
	if err != nil || count != 1 {
		t.Fatalf("approved shop rows = %d (err %v), want 1", count, err)
	}

	// its socials landed on the shop row
	var website, instagram sql.NullString
	err = db.QueryRow(
		`SELECT website, instagram_url FROM shops WHERE name = 'Integration Vélo'`,
	).Scan(&website, &instagram)
	if err != nil {
		t.Fatalf("shops socials: %v", err)
	}
	if website.String != "https://integration.example" || instagram.String != "https://instagram.com/integrationvelo" {
		t.Errorf("socials = %q, %q", website.String, instagram.String)
	}

	// approving twice conflicts
	resp, _ = do("POST", fmt.Sprintf("/api/submissions/%d/approve", sub.ID), adminCookie, "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("second approve = %d, want 404", resp.StatusCode)
	}
}
