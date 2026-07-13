package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/ireydiak/shops/internal/auth"
)

// userJSON is one role assignment. Source "config" entries come from
// ADMIN_EMAILS and cannot be revoked through the API.
type userJSON struct {
	Email     string     `json:"email"`
	Role      string     `json:"role"`
	Source    string     `json:"source"`
	CreatedAt *time.Time `json:"created_at"`
}

// listUsers handles GET /api/users (admin): configured admins first, then
// database role assignments.
func (s *server) listUsers(authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out := []userJSON{}
		for _, email := range authSvc.ConfigAdmins() {
			out = append(out, userJSON{Email: email, Role: "admin", Source: "config"})
		}
		rows, err := s.db.Query(`SELECT email, role, created_at FROM users ORDER BY email`)
		if err != nil {
			s.internalError(w, err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			u := userJSON{Source: "database"}
			var created time.Time
			if err := rows.Scan(&u.Email, &u.Role, &created); err != nil {
				s.internalError(w, err)
				return
			}
			u.CreatedAt = &created
			out = append(out, u)
		}
		if err := rows.Err(); err != nil {
			s.internalError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// putUser handles PUT /api/users/{email} (admin): grants or changes a role.
func (s *server) putUser(w http.ResponseWriter, r *http.Request) {
	email := strings.ToLower(strings.TrimSpace(r.PathValue("email")))
	if email == "" || !strings.Contains(email, "@") {
		badRequest(w, "invalid email")
		return
	}
	var body struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w, "invalid JSON: "+err.Error())
		return
	}
	if body.Role != "admin" && body.Role != "user" {
		badRequest(w, "role must be admin or user")
		return
	}
	var created time.Time
	err := s.db.QueryRow(
		`INSERT INTO users (email, role) VALUES ($1, $2::user_role)
		 ON CONFLICT (email) DO UPDATE SET role = EXCLUDED.role, updated_at = NOW()
		 RETURNING created_at`,
		email, body.Role,
	).Scan(&created)
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, userJSON{Email: email, Role: body.Role, Source: "database", CreatedAt: &created})
}

// deleteUser handles DELETE /api/users/{email} (admin): removes a database
// role assignment (configured admins are untouchable).
func (s *server) deleteUser(w http.ResponseWriter, r *http.Request) {
	email := strings.ToLower(strings.TrimSpace(r.PathValue("email")))
	res, err := s.db.Exec(`DELETE FROM users WHERE email = $1`, email)
	if err != nil {
		s.internalError(w, err)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no role assignment for that email"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// adminLookup is the database side of auth.Service.IsAdmin.
func adminLookup(db *sql.DB) func(string) (bool, error) {
	return func(email string) (bool, error) {
		var isAdmin bool
		err := db.QueryRow(
			`SELECT EXISTS (SELECT 1 FROM users WHERE email = $1 AND role = 'admin')`, email,
		).Scan(&isAdmin)
		return isAdmin, err
	}
}
