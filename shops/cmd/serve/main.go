package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/ireydiak/shops/internal/auth"
	"github.com/ireydiak/shops/internal/nominatim"
	"github.com/lib/pq"
)

//go:embed static
var static embed.FS

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}

	pages, err := fs.Sub(static, "static")
	if err != nil {
		log.Fatal(err)
	}
	authSvc, err := auth.NewFromEnv(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	authSvc.SetAdminLookup(adminLookup(db))

	s := &server{db: db, geo: nominatim.New()}
	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServerFS(pages))
	mux.HandleFunc("GET /api/shops", func(w http.ResponseWriter, r *http.Request) {
		rows, err := loadShops(db)
		if err != nil {
			log.Printf("loading shops: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/geo+json")
		if err := json.NewEncoder(w).Encode(toFeatureCollection(rows)); err != nil {
			log.Printf("encoding shops: %v", err)
		}
	})

	mux.HandleFunc("GET /api/shops/all", s.listAllShops)
	mux.HandleFunc("GET /api/shops/export.csv", s.exportShopsCSV)
	mux.HandleFunc("GET /api/shops/{id}", s.getShop)

	// auth
	mux.HandleFunc("GET /auth/{provider}/login", authSvc.Login)
	mux.HandleFunc("GET /auth/{provider}/callback", authSvc.Callback)
	mux.HandleFunc("POST /auth/logout", authSvc.Logout)
	mux.HandleFunc("GET /api/me", authSvc.Me)
	mux.HandleFunc("GET /api/auth/providers", authSvc.Providers)
	if authSvc.DevEnabled() {
		authSvc.RegisterDevIdP(mux)
	}

	// admin: direct writes and moderation
	mux.Handle("POST /api/shops", authSvc.RequireAdmin(http.HandlerFunc(s.createShop)))
	mux.Handle("PATCH /api/shops/{id}", authSvc.RequireAdmin(http.HandlerFunc(s.updateShop)))
	mux.Handle("GET /api/submissions", authSvc.RequireAdmin(http.HandlerFunc(s.listSubmissions)))
	mux.Handle("POST /api/submissions/{id}/approve", authSvc.RequireAdmin(s.reviewSubmission(true)))
	mux.Handle("POST /api/submissions/{id}/reject", authSvc.RequireAdmin(s.reviewSubmission(false)))
	mux.Handle("GET /api/users", authSvc.RequireAdmin(s.listUsers(authSvc)))
	mux.Handle("PUT /api/users/{email}", authSvc.RequireAdmin(http.HandlerFunc(s.putUser)))
	mux.Handle("DELETE /api/users/{email}", authSvc.RequireAdmin(http.HandlerFunc(s.deleteUser)))

	// any logged-in user: propose a change
	mux.Handle("POST /api/submissions", authSvc.RequireUser(http.HandlerFunc(s.createSubmission)))

	log.Printf("serving on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func loadShops(db *sql.DB) ([]shopRow, error) {
	rows, err := db.Query(
		`SELECT shop_id, name, status, street_number, street_name, city, postal_code,
		        ST_X(location::geometry), ST_Y(location::geometry), tags
		 FROM shops
		 WHERE location IS NOT NULL
		 ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shops []shopRow
	for rows.Next() {
		var r shopRow
		if err := rows.Scan(&r.ID, &r.Name, &r.Status, &r.StreetNumber, &r.StreetName, &r.City, &r.PostalCode, &r.Lon, &r.Lat, pq.Array(&r.Tags)); err != nil {
			return nil, err
		}
		shops = append(shops, r)
	}
	return shops, rows.Err()
}
