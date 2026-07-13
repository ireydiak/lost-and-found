package main

import (
	"database/sql"
	"log"
	"os"

	"github.com/ireydiak/shops/internal/nominatim"
	_ "github.com/lib/pq"
)

// addrRow is one addresses row waiting for a location.
type addrRow struct {
	ID int64
	nominatim.Address
}

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

	rows, err := db.Query(
		`SELECT address_id, street_number, street_name, city, COALESCE(borough, ''), postal_code
		 FROM addresses WHERE location IS NULL ORDER BY address_id`)
	if err != nil {
		log.Fatal(err)
	}
	var addrs []addrRow
	for rows.Next() {
		var a addrRow
		if err := rows.Scan(&a.ID, &a.StreetNumber, &a.StreetName, &a.City, &a.Borough, &a.PostalCode); err != nil {
			log.Fatal(err)
		}
		addrs = append(addrs, a)
	}
	if err := rows.Close(); err != nil {
		log.Fatal(err)
	}
	log.Printf("%d addresses without a location", len(addrs))

	g := nominatim.New()

	var done int
	var misses []addrRow
	for i, a := range addrs {
		pt, ok, err := g.Geocode(a.Address)
		if err != nil {
			log.Fatalf("address %d (%s %s): %v", a.ID, a.StreetNumber, a.StreetName, err)
		}
		if !ok {
			misses = append(misses, a)
			continue
		}
		_, err = db.Exec(
			`UPDATE addresses
			 SET location = ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, updated_at = NOW()
			 WHERE address_id = $3`,
			pt.Lon, pt.Lat, a.ID)
		if err != nil {
			log.Fatalf("address %d: %v", a.ID, err)
		}
		done++
		if (i+1)%25 == 0 {
			log.Printf("  %d/%d...", i+1, len(addrs))
		}
	}

	log.Printf("done: %d geocoded, %d not found", done, len(misses))
	for _, a := range misses {
		log.Printf("  miss: [%d] %s %s, %s %s", a.ID, a.StreetNumber, a.StreetName, a.City, a.PostalCode)
	}
}
