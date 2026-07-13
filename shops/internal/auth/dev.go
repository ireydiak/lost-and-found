package auth

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/oauth2"
)

// The dev provider is a miniature identity provider served by the application
// itself, so local development never depends on Google or Facebook. Any email
// typed into the form logs in — which is why EnableDevProvider refuses to run
// unless the app is bound to localhost.

// EnableDevProvider registers the "dev" OAuth provider against the app's own
// /devauth/* endpoints (mounted with RegisterDevIdP).
func (s *Service) EnableDevProvider() error {
	u, err := url.Parse(s.baseURL)
	if err != nil {
		return fmt.Errorf("auth: parsing base URL: %w", err)
	}
	if host := u.Hostname(); host != "localhost" && host != "127.0.0.1" {
		return fmt.Errorf("auth: the dev login provider requires a localhost BASE_URL, got %q", s.baseURL)
	}
	s.AddProvider("dev", Provider{
		Config: &oauth2.Config{
			ClientID:     "dev",
			ClientSecret: "dev",
			Endpoint: oauth2.Endpoint{
				AuthURL:  s.baseURL + "/devauth/authorize",
				TokenURL: s.baseURL + "/devauth/token",
			},
			Scopes: []string{"email"},
		},
		UserInfoURL: s.baseURL + "/devauth/userinfo",
	})
	return nil
}

// DevEnabled reports whether the dev provider is registered.
func (s *Service) DevEnabled() bool {
	_, ok := s.providers["dev"]
	return ok
}

// RegisterDevIdP mounts the mini identity provider.
func (s *Service) RegisterDevIdP(mux *http.ServeMux) {
	mux.HandleFunc("GET /devauth/authorize", s.devAuthorizeForm)
	mux.HandleFunc("POST /devauth/authorize", s.devAuthorize)
	mux.HandleFunc("POST /devauth/token", devToken)
	mux.HandleFunc("GET /devauth/userinfo", devUserInfo)
}

var devForm = template.Must(template.New("devauth").Parse(`<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>Dev sign-in</title>
<style>
  body { font-family: system-ui, sans-serif; display: grid; place-items: center; min-height: 90vh; background: #f5f5f5; }
  form { background: #fff; padding: 28px 32px; border-radius: 10px; box-shadow: 0 2px 8px rgba(0,0,0,.12); display: grid; gap: 12px; width: 300px; }
  h1 { font-size: 17px; margin: 0; }
  p { margin: 0; color: #b26a00; font-size: 13px; }
  input[type=email] { padding: 8px 10px; border: 1px solid #ccc; border-radius: 6px; font: inherit; }
  button { padding: 9px; border: none; border-radius: 6px; background: #1565c0; color: #fff; font: inherit; cursor: pointer; }
</style></head>
<body>
<form method="post" action="/devauth/authorize">
  <h1>Dev sign-in</h1>
  <p>Local development only — any email works.</p>
  <input type="hidden" name="state" value="{{.State}}">
  <input type="hidden" name="redirect_uri" value="{{.RedirectURI}}">
  <input type="email" name="email" placeholder="you@example.com" required autofocus>
  <button>Sign in</button>
</form>
</body></html>`))

func (s *Service) devAuthorizeForm(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	devForm.Execute(w, map[string]string{
		"State":       r.URL.Query().Get("state"),
		"RedirectURI": r.URL.Query().Get("redirect_uri"),
	})
}

func (s *Service) devAuthorize(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(r.FormValue("email"))
	redirectURI := r.FormValue("redirect_uri")
	target, err := url.Parse(redirectURI)
	if err != nil || email == "" || !strings.HasPrefix(redirectURI, s.baseURL+"/") {
		http.Error(w, "bad redirect_uri or email", http.StatusBadRequest)
		return
	}
	q := target.Query()
	q.Set("code", email) // dev only: the "authorization code" is the email itself
	q.Set("state", r.FormValue("state"))
	target.RawQuery = q.Encode()
	http.Redirect(w, r, target.String(), http.StatusFound)
}

func devToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"access_token": r.FormValue("code"),
		"token_type":   "Bearer",
	})
}

func devUserInfo(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if email == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"email": email})
}
