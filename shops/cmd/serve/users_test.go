package main

import (
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/ireydiak/shops/internal/auth"
)

// TestRoleGrantIntegration grants a database role through the API and proves
// the grantee can then pass RequireAdmin — and no longer can after revocation.
// Skipped unless DATABASE_URL is set.
func TestRoleGrantIntegration(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Ping(); err != nil {
		t.Skipf("database not reachable: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`DELETE FROM users WHERE email LIKE '%@roles.test'`); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})

	authSvc, err := auth.NewService(auth.Config{
		Secret: "roles-test", BaseURL: "http://localhost:8080",
		Admins: []string{"boot@roles.test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	authSvc.SetAdminLookup(adminLookup(db))
	s := &server{db: db}

	mux := http.NewServeMux()
	mux.Handle("GET /api/users", authSvc.RequireAdmin(s.listUsers(authSvc)))
	mux.Handle("PUT /api/users/{email}", authSvc.RequireAdmin(http.HandlerFunc(s.putUser)))
	mux.Handle("DELETE /api/users/{email}", authSvc.RequireAdmin(http.HandlerFunc(s.deleteUser)))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	bootCookie, _ := authSvc.MintSession("boot@roles.test")
	newCookie, _ := authSvc.MintSession("promoted@roles.test")

	do := func(method, path string, cookie *http.Cookie, body string) (int, string) {
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
		out, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(out)
	}

	// not an admin yet: the new user is refused
	if code, _ := do("GET", "/api/users", newCookie, ""); code != http.StatusForbidden {
		t.Fatalf("pre-grant admin access = %d, want 403", code)
	}

	// the bootstrap admin grants the role
	code, body := do("PUT", "/api/users/Promoted@roles.test", bootCookie, `{"role": "admin"}`)
	if code != http.StatusOK || !strings.Contains(body, `"promoted@roles.test"`) {
		t.Fatalf("grant = %d: %s", code, body)
	}

	// the grantee now passes RequireAdmin and sees both sources in the list
	code, body = do("GET", "/api/users", newCookie, "")
	if code != http.StatusOK {
		t.Fatalf("post-grant admin access = %d: %s", code, body)
	}
	for _, want := range []string{`"source":"config"`, `"source":"database"`, "boot@roles.test", "promoted@roles.test"} {
		if !strings.Contains(body, want) {
			t.Errorf("user list missing %s: %s", want, body)
		}
	}

	// revoke, and access disappears
	if code, body = do("DELETE", "/api/users/promoted@roles.test", bootCookie, ""); code != http.StatusNoContent {
		t.Fatalf("revoke = %d: %s", code, body)
	}
	if code, _ = do("GET", "/api/users", newCookie, ""); code != http.StatusForbidden {
		t.Fatalf("post-revoke admin access = %d, want 403", code)
	}

	// revoking a non-existent assignment 404s
	if code, _ = do("DELETE", "/api/users/ghost@roles.test", bootCookie, ""); code != http.StatusNotFound {
		t.Errorf("revoke missing = %d, want 404", code)
	}
}
