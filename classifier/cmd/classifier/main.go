package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq"

	"classifier/internal/web"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	appEnv := os.Getenv("APP_ENV")
	isDev := appEnv == "development"

	server, err := web.NewServer(db, isDev)
	if err != nil {
		log.Fatalf("failed to initialize server: %v", err)
	}

	addr := ":8080"
	log.Printf("listening on %s (APP_ENV=%q)", addr, appEnv)
	log.Fatal(http.ListenAndServe(addr, server.Routes()))
}
