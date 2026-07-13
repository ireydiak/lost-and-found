package main

import (
	"archive/zip"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

func main() {
	zipPath := flag.String("zip", "", "path to the REQ open-data ZIP (JeuDonnees.zip)")
	dryRun := flag.Bool("dry-run", false, "write a preview CSV instead of touching the database")
	previewPath := flag.String("preview", "data/import-preview.csv", "preview CSV path for -dry-run")
	flag.Parse()

	if *zipPath == "" {
		log.Fatal("usage: import-req -zip path/to/JeuDonnees.zip [-dry-run]")
	}

	z, err := zip.OpenReader(*zipPath)
	if err != nil {
		log.Fatalf("opening zip: %v", err)
	}
	defer z.Close()

	log.Println("pass 1/3: establishments...")
	estabs, err := loadEstablishments(z)
	if err != nil {
		log.Fatal(err)
	}
	cands := make(map[string]*Candidate)
	for neq, list := range estabs {
		for _, e := range list {
			if len(e.MatchTags) > 0 {
				if cands[neq] == nil {
					cands[neq] = &Candidate{}
				}
			}
		}
	}
	log.Printf("  %d enterprises with establishments, %d candidates so far", len(estabs), len(cands))

	log.Println("pass 2/3: enterprises (status, domicile, CAE)...")
	if err := scanEnterprises(z, cands); err != nil {
		log.Fatal(err)
	}
	log.Printf("  %d candidates", len(cands))

	log.Println("pass 3/3: names (legal and trade names)...")
	if err := collectNames(z, cands); err != nil {
		log.Fatal(err)
	}

	rows := buildRows(cands, estabs)
	log.Printf("%d shop rows on the Island of Montreal", len(rows))

	if *dryRun {
		if err := writePreview(*previewPath, rows); err != nil {
			log.Fatal(err)
		}
		log.Printf("dry run: preview written to %s", *previewPath)
		return
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}
	inserted, updated, err := upsertRows(dsn, rows)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("done: %d inserted, %d updated", inserted, updated)
}

func writePreview(path string, rows []ShopRow) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write([]string{"neq", "etab_no", "name", "status", "street_number", "street_name", "city", "borough", "postal_code", "tags"}); err != nil {
		return err
	}
	for _, r := range rows {
		err := w.Write([]string{
			r.NEQ, strconv.Itoa(r.EtabNo), r.Name, r.Status,
			r.Addr.StreetNumber, r.Addr.StreetName, r.Addr.City, r.Addr.Borough, r.Addr.PostalCode,
			strings.Join(r.Tags, "|"),
		})
		if err != nil {
			return err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	fmt.Println() // keep log output separated from the summary
	return nil
}
