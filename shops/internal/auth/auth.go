// Package auth provides OAuth2 login (Google, Facebook) with signed-cookie
// sessions. Nothing is stored server-side: the session cookie carries only
// the user's email, and admin rights come from configuration.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/securecookie"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/facebook"
	"golang.org/x/oauth2/google"
)

const (
	sessionCookieName = "session"
	stateCookieName   = "oauth_state"
	sessionTTL        = 24 * time.Hour
)

// Provider is one OAuth2 identity provider plus the endpoint that reveals the
// logged-in user's email.
type Provider struct {
	Config      *oauth2.Config
	UserInfoURL string
}

// Config configures a Service.
type Config struct {
	Secret  string   // signs and encrypts session cookies
	BaseURL string   // public origin, used to build callback URLs
	Admins  []string // emails allowed on admin routes
}

// Service issues sessions and guards routes.
type Service struct {
	cookies     *securecookie.SecureCookie
	baseURL     string
	secure      bool
	admins      map[string]bool
	lookupAdmin func(email string) (bool, error)
	providers   map[string]Provider
}

func NewService(cfg Config) (*Service, error) {
	if cfg.Secret == "" {
		return nil, fmt.Errorf("auth: session secret is required")
	}
	hashKey := sha256.Sum256([]byte(cfg.Secret + ":hash"))
	blockKey := sha256.Sum256([]byte(cfg.Secret + ":block"))
	sc := securecookie.New(hashKey[:], blockKey[:])
	sc.MaxAge(int(sessionTTL.Seconds()))

	admins := make(map[string]bool, len(cfg.Admins))
	for _, a := range cfg.Admins {
		if a = strings.ToLower(strings.TrimSpace(a)); a != "" {
			admins[a] = true
		}
	}
	return &Service{
		cookies:   sc,
		baseURL:   strings.TrimSuffix(cfg.BaseURL, "/"),
		secure:    strings.HasPrefix(cfg.BaseURL, "https://"),
		admins:    admins,
		providers: map[string]Provider{},
	}, nil
}

// NewFromEnv builds a Service from SESSION_SECRET, BASE_URL, ADMIN_EMAILS,
// GOOGLE_CLIENT_ID/SECRET and FACEBOOK_CLIENT_ID/SECRET. Providers with no
// client ID are simply not registered.
func NewFromEnv(getenv func(string) string) (*Service, error) {
	secret := getenv("SESSION_SECRET")
	if secret == "" {
		buf := make([]byte, 32)
		rand.Read(buf)
		secret = hex.EncodeToString(buf)
		log.Println("auth: SESSION_SECRET is not set; using a random one (sessions will not survive a restart)")
	}
	baseURL := getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	svc, err := NewService(Config{
		Secret:  secret,
		BaseURL: baseURL,
		Admins:  strings.Split(getenv("ADMIN_EMAILS"), ","),
	})
	if err != nil {
		return nil, err
	}
	if id := getenv("GOOGLE_CLIENT_ID"); id != "" {
		svc.AddProvider("google", Provider{
			Config: &oauth2.Config{
				ClientID:     id,
				ClientSecret: getenv("GOOGLE_CLIENT_SECRET"),
				Endpoint:     google.Endpoint,
				Scopes:       []string{"openid", "email"},
			},
			UserInfoURL: "https://openidconnect.googleapis.com/v1/userinfo",
		})
	}
	if id := getenv("FACEBOOK_CLIENT_ID"); id != "" {
		svc.AddProvider("facebook", Provider{
			Config: &oauth2.Config{
				ClientID:     id,
				ClientSecret: getenv("FACEBOOK_CLIENT_SECRET"),
				Endpoint:     facebook.Endpoint,
				Scopes:       []string{"email"},
			},
			UserInfoURL: "https://graph.facebook.com/v19.0/me?fields=email",
		})
	}
	if getenv("DEV_AUTH") == "1" {
		if err := svc.EnableDevProvider(); err != nil {
			return nil, err
		}
		log.Println("auth: DEV LOGIN ENABLED — any email can sign in; never set DEV_AUTH outside local development")
	}
	if len(svc.providers) == 0 {
		log.Println("auth: no OAuth providers configured (set GOOGLE_CLIENT_ID / FACEBOOK_CLIENT_ID, or DEV_AUTH=1 locally); login is unavailable")
	}
	return svc, nil
}

// ProviderNames returns the registered provider names, sorted.
func (s *Service) ProviderNames() []string {
	names := make([]string, 0, len(s.providers))
	for n := range s.providers {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Providers handles GET /api/auth/providers so pages can render the right
// sign-in buttons.
func (s *Service) Providers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.ProviderNames())
}

// AddProvider registers a provider under a URL name ("google") and pins its
// redirect URL to BASE_URL/auth/{name}/callback.
func (s *Service) AddProvider(name string, p Provider) {
	p.Config.RedirectURL = s.baseURL + "/auth/" + name + "/callback"
	s.providers[name] = p
}

// SetAdminLookup installs an additional admin source, typically a database.
// Emails on the configured list stay admins regardless, so the list acts as a
// bootstrap and a recovery override.
func (s *Service) SetAdminLookup(fn func(email string) (bool, error)) {
	s.lookupAdmin = fn
}

// ConfigAdmins returns the emails configured as admins (the bootstrap list).
func (s *Service) ConfigAdmins() []string {
	out := make([]string, 0, len(s.admins))
	for e := range s.admins {
		out = append(out, e)
	}
	sort.Strings(out)
	return out
}

// IsAdmin reports whether an email is an admin: on the configured list, or
// granted by the lookup. Lookup errors fail closed.
func (s *Service) IsAdmin(email string) bool {
	email = strings.ToLower(email)
	if s.admins[email] {
		return true
	}
	if s.lookupAdmin == nil {
		return false
	}
	ok, err := s.lookupAdmin(email)
	if err != nil {
		log.Printf("auth: admin lookup for %s: %v", email, err)
		return false
	}
	return ok
}

// Login handles GET /auth/{provider}/login.
func (s *Service) Login(w http.ResponseWriter, r *http.Request) {
	p, ok := s.providers[r.PathValue("provider")]
	if !ok {
		http.Error(w, "unknown provider", http.StatusNotFound)
		return
	}
	buf := make([]byte, 16)
	rand.Read(buf)
	state := hex.EncodeToString(buf)
	http.SetCookie(w, &http.Cookie{
		Name: stateCookieName, Value: state, Path: "/auth",
		MaxAge: 600, HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, p.Config.AuthCodeURL(state), http.StatusFound)
}

// Callback handles GET /auth/{provider}/callback.
func (s *Service) Callback(w http.ResponseWriter, r *http.Request) {
	p, ok := s.providers[r.PathValue("provider")]
	if !ok {
		http.Error(w, "unknown provider", http.StatusNotFound)
		return
	}
	stateCookie, err := r.Cookie(stateCookieName)
	if err != nil || stateCookie.Value == "" || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "state mismatch", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: stateCookieName, Path: "/auth", MaxAge: -1})

	token, err := p.Config.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		log.Printf("auth: code exchange: %v", err)
		http.Error(w, "login failed", http.StatusBadGateway)
		return
	}
	email, err := fetchEmail(r.Context(), p, token)
	if err != nil {
		log.Printf("auth: userinfo: %v", err)
		http.Error(w, "login failed", http.StatusBadGateway)
		return
	}

	cookie, err := s.MintSession(email)
	if err != nil {
		log.Printf("auth: encoding session: %v", err)
		http.Error(w, "login failed", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, cookie)
	http.Redirect(w, r, "/", http.StatusFound)
}

func fetchEmail(ctx context.Context, p Provider, token *oauth2.Token) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.UserInfoURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("userinfo: HTTP %s", resp.Status)
	}
	var info struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", err
	}
	if info.Email == "" {
		return "", fmt.Errorf("provider returned no email")
	}
	return strings.ToLower(info.Email), nil
}

// MintSession creates a signed session cookie for an email. Exported for
// integration tests.
func (s *Service) MintSession(email string) (*http.Cookie, error) {
	value, err := s.cookies.Encode(sessionCookieName, map[string]string{"email": email})
	if err != nil {
		return nil, err
	}
	return &http.Cookie{
		Name: sessionCookieName, Value: value, Path: "/",
		MaxAge: int(sessionTTL.Seconds()), HttpOnly: true,
		Secure: s.secure, SameSite: http.SameSiteLaxMode,
	}, nil
}

func (s *Service) sessionEmail(r *http.Request) (string, bool) {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return "", false
	}
	var value map[string]string
	if err := s.cookies.Decode(sessionCookieName, c.Value, &value); err != nil {
		return "", false
	}
	return value["email"], value["email"] != ""
}

// Logout handles POST /auth/logout.
func (s *Service) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/", http.StatusFound)
}

// Me handles GET /api/me.
func (s *Service) Me(w http.ResponseWriter, r *http.Request) {
	email, ok := s.sessionEmail(r)
	if !ok {
		unauthorized(w)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"email": email, "admin": s.IsAdmin(email)})
}

type contextKey struct{}

// EmailFromContext returns the logged-in email placed by the middleware.
func EmailFromContext(ctx context.Context) string {
	email, _ := ctx.Value(contextKey{}).(string)
	return email
}

// RequireUser lets any logged-in user through.
func (s *Service) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := s.sessionEmail(r)
		if !ok {
			unauthorized(w)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, email)))
	})
}

// RequireAdmin lets only configured admins through.
func (s *Service) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := s.sessionEmail(r)
		if !ok {
			unauthorized(w)
			return
		}
		if !s.IsAdmin(email) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "admin access required"})
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, email)))
	})
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{"error": "login required"})
}
