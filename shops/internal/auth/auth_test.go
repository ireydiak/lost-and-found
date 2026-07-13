package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

// fakeIdP is a minimal OAuth2 provider: it accepts any code at /token and
// returns the configured email at /userinfo.
func fakeIdP(t *testing.T, email string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"fake-token","token_type":"Bearer"}`)
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer fake-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"email": email})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func testService(t *testing.T, idp *httptest.Server, admins ...string) *Service {
	t.Helper()
	svc, err := NewService(Config{
		Secret:  "test-secret",
		BaseURL: "http://localhost:8080",
		Admins:  admins,
	})
	if err != nil {
		t.Fatal(err)
	}
	svc.AddProvider("test", Provider{
		Config: &oauth2.Config{
			ClientID:     "id",
			ClientSecret: "secret",
			Endpoint: oauth2.Endpoint{
				AuthURL:  idp.URL + "/auth",
				TokenURL: idp.URL + "/token",
			},
			Scopes: []string{"email"},
		},
		UserInfoURL: idp.URL + "/userinfo",
	})
	return svc
}

// loginAndCallback walks the full flow and returns the session cookie.
func loginAndCallback(t *testing.T, svc *Service) *http.Cookie {
	t.Helper()
	// Step 1: login redirects to the provider with a state cookie.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/auth/test/login", nil)
	req.SetPathValue("provider", "test")
	svc.Login(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("login status = %d", rec.Code)
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	state := loc.Query().Get("state")
	if state == "" {
		t.Fatal("no state in redirect")
	}
	var stateCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == stateCookieName {
			stateCookie = c
		}
	}
	if stateCookie == nil {
		t.Fatal("no state cookie set")
	}

	// Step 2: callback with the code and matching state.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/auth/test/callback?code=any&state="+state, nil)
	req.SetPathValue("provider", "test")
	req.AddCookie(stateCookie)
	svc.Callback(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("callback status = %d, body %s", rec.Code, rec.Body)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookieName && c.Value != "" {
			return c
		}
	}
	t.Fatal("no session cookie set")
	return nil
}

func TestFullLoginFlow(t *testing.T) {
	svc := testService(t, fakeIdP(t, "rider@example.com"))
	session := loginAndCallback(t, svc)

	req := httptest.NewRequest("GET", "/api/me", nil)
	req.AddCookie(session)
	rec := httptest.NewRecorder()
	svc.Me(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("me status = %d", rec.Code)
	}
	var me struct {
		Email string `json:"email"`
		Admin bool   `json:"admin"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&me); err != nil {
		t.Fatal(err)
	}
	if me.Email != "rider@example.com" || me.Admin {
		t.Errorf("me = %+v, want non-admin rider@example.com", me)
	}
}

func TestCallbackRejectsBadState(t *testing.T) {
	svc := testService(t, fakeIdP(t, "rider@example.com"))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/auth/test/callback?code=any&state=forged", nil)
	req.SetPathValue("provider", "test")
	svc.Callback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("forged state status = %d, want 400", rec.Code)
	}
}

func TestMiddleware(t *testing.T) {
	svc := testService(t, fakeIdP(t, "admin@example.com"), "admin@example.com")
	adminSession := loginAndCallback(t, svc)

	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "email="+EmailFromContext(r.Context()))
	})

	// anonymous is rejected by both
	for name, h := range map[string]http.Handler{
		"RequireUser":  svc.RequireUser(okHandler),
		"RequireAdmin": svc.RequireAdmin(okHandler),
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("POST", "/x", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s anonymous = %d, want 401", name, rec.Code)
		}
	}

	// admin passes both, and the email lands in the context
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/x", nil)
	req.AddCookie(adminSession)
	svc.RequireAdmin(okHandler).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "admin@example.com") {
		t.Errorf("admin request = %d %q", rec.Code, rec.Body)
	}

	// a non-admin user passes RequireUser but not RequireAdmin
	svc2 := testService(t, fakeIdP(t, "rider@example.com"), "admin@example.com")
	userSession := loginAndCallback(t, svc2)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/x", nil)
	req.AddCookie(userSession)
	svc2.RequireUser(okHandler).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("user RequireUser = %d, want 200", rec.Code)
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/x", nil)
	req.AddCookie(userSession)
	svc2.RequireAdmin(okHandler).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("user RequireAdmin = %d, want 403", rec.Code)
	}
}

func TestIsAdminWithLookup(t *testing.T) {
	svc, err := NewService(Config{Secret: "s", BaseURL: "http://localhost:8080", Admins: []string{"config@x.y"}})
	if err != nil {
		t.Fatal(err)
	}
	svc.SetAdminLookup(func(email string) (bool, error) {
		switch email {
		case "db@x.y":
			return true, nil
		case "boom@x.y":
			return false, fmt.Errorf("db down")
		}
		return false, nil
	})
	cases := []struct {
		email string
		want  bool
	}{
		{"config@x.y", true},  // env list wins without consulting the lookup
		{"Config@X.Y", true},  // case-insensitive
		{"db@x.y", true},      // granted in the database
		{"nobody@x.y", false}, // no role anywhere
		{"boom@x.y", false},   // lookup errors fail closed
	}
	for _, c := range cases {
		if got := svc.IsAdmin(c.email); got != c.want {
			t.Errorf("IsAdmin(%q) = %v, want %v", c.email, got, c.want)
		}
	}
}

func TestForgedSessionCookieRejected(t *testing.T) {
	svc := testService(t, fakeIdP(t, "rider@example.com"))
	req := httptest.NewRequest("GET", "/api/me", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "forged-garbage"})
	rec := httptest.NewRecorder()
	svc.Me(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("forged cookie = %d, want 401", rec.Code)
	}
}

func TestLogoutClearsSession(t *testing.T) {
	svc := testService(t, fakeIdP(t, "rider@example.com"))
	rec := httptest.NewRecorder()
	svc.Logout(rec, httptest.NewRequest("POST", "/auth/logout", nil))
	var cleared bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookieName && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("logout did not clear the session cookie")
	}
}
