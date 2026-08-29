package main

import (
	"database/sql"
	"encoding/csv"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/lib/pq"
)

// exportRow is one shop joined with its address, as read for CSV export.
type exportRow struct {
	Name         string
	Status       string
	StreetNumber string
	StreetName   string
	City         string
	PostalCode   string
	Phone        *string
	Email        *string
	Website      *string
	NeqID        *string
	NeqEtabID    int
	Tags         []string
}

var accentFold = strings.NewReplacer(
	"à", "a", "â", "a", "ä", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"î", "i", "ï", "i",
	"ô", "o", "ö", "o",
	"ù", "u", "û", "u", "ü", "u",
	"ç", "c",
	"œ", "oe",
)

// fold matches shops.html's client-side search normalization, so the export
// selects the same rows the page shows.
func fold(s string) string {
	return accentFold.Replace(strings.ToLower(s))
}

// validCategories mirrors the Shops/Community toggle in shops.html: a shop
// is non-profit if tagged as such, for-profit otherwise.
var validCategories = map[string]bool{"for-profit": true, "non-profit": true}

// filterExportRows applies the status/category/search filters shops.html
// applies client-side. A nil statuses or categories map means "no filter on
// that dimension"; an empty q means no search filter.
func filterExportRows(rows []exportRow, statuses, categories map[string]bool, q string) []exportRow {
	fq := fold(q)
	out := []exportRow{}
	for _, r := range rows {
		if statuses != nil && !statuses[r.Status] {
			continue
		}
		category := "for-profit"
		if slices.Contains(r.Tags, "non-profit") {
			category = "non-profit"
		}
		if categories != nil && !categories[category] {
			continue
		}
		if fq != "" && !strings.Contains(fold(r.Name), fq) {
			continue
		}
		out = append(out, r)
	}
	return out
}

var csvHeader = []string{"Name", "Address", "Phone", "Email", "Website", "NEQ ID", "NEQ Establishment ID"}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// writeCSV writes the export header and one row per shop.
func writeCSV(w interface{ Write([]byte) (int, error) }, rows []exportRow) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(csvHeader); err != nil {
		return err
	}
	for _, r := range rows {
		neqID, neqEtabID := "", ""
		if r.NeqID != nil {
			neqID = *r.NeqID
			neqEtabID = strconv.Itoa(r.NeqEtabID)
		}
		record := []string{
			r.Name,
			formatAddress(r.StreetNumber, r.StreetName, r.City, r.PostalCode),
			deref(r.Phone),
			deref(r.Email),
			deref(r.Website),
			neqID,
			neqEtabID,
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

const exportSelect = `
	SELECT name, status, street_number, street_name, city, postal_code,
	       phone, email, website, neq_id, neq_etab_id, tags
	FROM shop_details
	ORDER BY name`

// loadExportRows reads every shop for CSV export; filtering happens in Go so
// it can share logic with the tests instead of building dynamic SQL.
func loadExportRows(db *sql.DB) ([]exportRow, error) {
	rows, err := db.Query(exportSelect)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []exportRow
	for rows.Next() {
		var (
			r                            exportRow
			street, city, postal         sql.NullString
			phone, email, website, neqID sql.NullString
		)
		if err := rows.Scan(&r.Name, &r.Status, &r.StreetNumber, &street, &city, &postal,
			&phone, &email, &website, &neqID, &r.NeqEtabID, pq.Array(&r.Tags)); err != nil {
			return nil, err
		}
		r.StreetName = street.String
		r.City = city.String
		r.PostalCode = postal.String
		if phone.Valid {
			r.Phone = &phone.String
		}
		if email.Valid {
			r.Email = &email.String
		}
		if website.Valid {
			r.Website = &website.String
		}
		if neqID.Valid {
			r.NeqID = &neqID.String
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// parseFilterSet turns a comma-separated query param into a lookup set,
// validated against allowed. A missing/empty param means "no filter" (nil).
func parseFilterSet(raw string, allowed map[string]bool) (map[string]bool, error) {
	if raw == "" {
		return nil, nil
	}
	set := map[string]bool{}
	for _, v := range strings.Split(raw, ",") {
		v = strings.TrimSpace(v)
		if !allowed[v] {
			return nil, errInvalidFilterValue(v)
		}
		set[v] = true
	}
	return set, nil
}

type errInvalidFilterValue string

func (e errInvalidFilterValue) Error() string { return "invalid filter value: " + string(e) }

// exportShopsCSV handles GET /api/shops/export.csv (public): the shops
// matching the status/category/q filters, as a CSV download.
func (s *server) exportShopsCSV(w http.ResponseWriter, r *http.Request) {
	statuses, err := parseFilterSet(r.URL.Query().Get("status"), validStatus)
	if err != nil {
		badRequest(w, err.Error())
		return
	}
	categories, err := parseFilterSet(r.URL.Query().Get("category"), validCategories)
	if err != nil {
		badRequest(w, err.Error())
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))

	rows, err := loadExportRows(s.db)
	if err != nil {
		s.internalError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="shops.csv"`)
	if err := writeCSV(w, filterExportRows(rows, statuses, categories, q)); err != nil {
		log.Printf("writing shops export: %v", err)
	}
}
