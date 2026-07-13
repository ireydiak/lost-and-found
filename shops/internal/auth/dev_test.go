package auth

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// TestDevProviderFullFlow walks the entire dev login exactly as a browser
// would: login redirect → email form → authorize → callback → session.
func TestDevProviderFullFlow(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	svc, err := NewService(Config{Secret: "test", BaseURL: srv.URL, Admins: []string{"dev-admin@local.test"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.EnableDevProvider(); err != nil {
		t.Fatal(err)
	}
	svc.RegisterDevIdP(mux)
	mux.HandleFunc("GET /auth/{provider}/login", svc.Login)
	mux.HandleFunc("GET /auth/{provider}/callback", svc.Callback)
	mux.HandleFunc("GET /api/me", svc.Me)

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	// login → provider authorize form
	resp, err := client.Get(srv.URL + "/auth/dev/login")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `name="email"`) {
		t.Fatalf("authorize form: status %d body %.200s", resp.StatusCode, body)
	}
	// pull the hidden state/redirect_uri out of the form
	pick := func(name string) string {
		m := regexp.MustCompile(`name="` + name + `" value="([^"]*)"`).FindStringSubmatch(string(body))
		if m == nil {
			t.Fatalf("form is missing %s: %.300s", name, body)
		}
		return m[1]
	}
	state, redirectURI := pick("state"), pick("redirect_uri")

	// submit the email → callback → home, with a session cookie set
	resp, err = client.PostForm(srv.URL+"/devauth/authorize", url.Values{
		"email":        {"Dev-Admin@local.test"},
		"state":        {state},
		"redirect_uri": {redirectURI},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// session is live and the email was normalized to lower case
	resp, err = client.Get(srv.URL + "/api/me")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	me, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("me after dev login: %d %s", resp.StatusCode, me)
	}
	for _, want := range []string{`"email":"dev-admin@local.test"`, `"admin":true`} {
		if !strings.Contains(string(me), want) {
			t.Errorf("me = %s, missing %s", me, want)
		}
	}
}

func TestDevProviderRequiresLocalhost(t *testing.T) {
	svc, err := NewService(Config{Secret: "test", BaseURL: "https://shops.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.EnableDevProvider(); err == nil {
		t.Fatal("expected non-localhost base URL to be rejected")
	}
}

func TestDevAuthorizeRejectsForeignRedirect(t *testing.T) {
	svc, err := NewService(Config{Secret: "test", BaseURL: "http://localhost:8080"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.EnableDevProvider(); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/devauth/authorize", strings.NewReader(url.Values{
		"email":        {"x@y.z"},
		"state":        {"s"},
		"redirect_uri": {"https://evil.example/steal"},
	}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	svc.devAuthorize(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("foreign redirect_uri = %d, want 400", rec.Code)
	}
}
