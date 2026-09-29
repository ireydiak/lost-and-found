package main

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
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

	goose.SetDialect("postgres")

	if err := goose.Up(db, "db/migrations"); err != nil {
		log.Fatalf("migrations failed: %v", err)
	}

	// Seeds are tracked in a separate goose version table so their applied
	// state doesn't interleave with schema migrations.
	goose.SetTableName("goose_db_version_seeds")
	if err := goose.Up(db, "db/seeds"); err != nil {
		log.Fatalf("seeds failed: %v", err)
	}
}
