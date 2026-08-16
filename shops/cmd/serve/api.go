package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/ireydiak/shops/internal/nominatim"
	"github.com/lib/pq"
)

type server struct {
	db  *sql.DB
	geo *nominatim.Geocoder
}

type locationPayload struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type addressPayload struct {
	StreetNumber *string `json:"street_number"`
	StreetName   *string `json:"street_name"`
	City         *string `json:"city"`
	Borough      *string `json:"borough"`
	PostalCode   *string `json:"postal_code"`
}

// shopPayload is the body of POST and PATCH requests; absent fields are nil
// and, on PATCH, left untouched.
type shopPayload struct {
	Name         *string          `json:"name"`
	Status       *string          `json:"status"`
	Phone        *string          `json:"phone"`
	Email        *string          `json:"email"`
	Website      *string          `json:"website"`
	InstagramURL *string          `json:"instagram_url"`
	FacebookURL  *string          `json:"facebook_url"`
	Address      *addressPayload  `json:"address"`
	Location     *locationPayload `json:"location"`
}

var validStatus = map[string]bool{"active": true, "inactive": true, "closed": true}

func (p *shopPayload) validate(forCreate bool) error {
	if forCreate {
		if p.Name == nil {
			return errors.New("name is required")
		}
		if p.Address == nil {
			return errors.New("address is required")
		}
		required := []struct {
			field string
			v     *string
		}{
			{"street_number", p.Address.StreetNumber},
			{"street_name", p.Address.StreetName},
			{"city", p.Address.City},
			{"postal_code", p.Address.PostalCode},
		}
		for _, r := range required {
			if r.v == nil || strings.TrimSpace(*r.v) == "" {
				return fmt.Errorf("address.%s is required", r.field)
			}
		}
	} else if *p == (shopPayload{}) {
		return errors.New("at least one field must be provided")
	}
	if p.Name != nil && strings.TrimSpace(*p.Name) == "" {
		return errors.New("name cannot be empty")
	}
	if p.Status != nil && !validStatus[*p.Status] {
		return errors.New("status must be one of active, inactive, closed")
	}
	if p.Location != nil && !nominatim.InBounds(p.Location.Lat, p.Location.Lon) {
		return errors.New("location is outside the Montreal area")
	}
	return nil
}

func (a *addressPayload) toNominatim() nominatim.Address {
	deref := func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	}
	return nominatim.Address{
		StreetNumber: deref(a.StreetNumber),
		StreetName:   deref(a.StreetName),
		City:         deref(a.City),
		Borough:      deref(a.Borough),
		PostalCode:   deref(a.PostalCode),
	}
}

// shopJSON is the API representation of a shop.
type shopJSON struct {
	ID           int64            `json:"id"`
	Name         string           `json:"name"`
	Status       string           `json:"status"`
	Phone        *string          `json:"phone"`
	Email        *string          `json:"email"`
	Website      *string          `json:"website"`
	InstagramURL *string          `json:"instagram_url"`
	FacebookURL  *string          `json:"facebook_url"`
	Address      *addressJSON     `json:"address"`
	Location     *locationPayload `json:"location"`
	Tags         []string         `json:"tags"`
}

type addressJSON struct {
	StreetNumber string  `json:"street_number"`
	StreetName   string  `json:"street_name"`
	City         string  `json:"city"`
	Borough      *string `json:"borough"`
	PostalCode   string  `json:"postal_code"`
}

func badRequest(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encoding response: %v", err)
	}
}

func (s *server) internalError(w http.ResponseWriter, err error) {
	log.Printf("api: %v", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}

// createShop handles POST /api/shops.
func (s *server) createShop(w http.ResponseWriter, r *http.Request) {
	var p shopPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		badRequest(w, "invalid JSON: "+err.Error())
		return
	}
	if err := p.validate(true); err != nil {
		badRequest(w, err.Error())
		return
	}
	shopID, err := s.insertShop(&p)
	if err != nil {
		s.internalError(w, err)
		return
	}
	s.respondWithShop(w, http.StatusCreated, shopID)
}

// insertShop creates the address and shop rows for a validated create
// payload, geocoding the address when no explicit location is given.
func (s *server) insertShop(p *shopPayload) (int64, error) {
	loc := p.Location
	if loc == nil {
		pt, ok, err := s.geo.Geocode(p.Address.toNominatim())
		if err != nil {
			log.Printf("geocoding new shop: %v", err) // shop is still created, location stays empty
		} else if ok {
			loc = &locationPayload{Lat: pt.Lat, Lon: pt.Lon}
		}
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var lat, lon sql.NullFloat64
	if loc != nil {
		lat = sql.NullFloat64{Float64: loc.Lat, Valid: true}
		lon = sql.NullFloat64{Float64: loc.Lon, Valid: true}
	}
	addr := p.Address.toNominatim()
	var addressID int64
	err = tx.QueryRow(
		`INSERT INTO addresses (street_number, street_name, city, borough, postal_code, location)
		 VALUES ($1, $2, $3, NULLIF($4, ''), $5, ST_SetSRID(ST_MakePoint($6, $7), 4326)::geography)
		 RETURNING address_id`,
		addr.StreetNumber, addr.StreetName, addr.City, addr.Borough, addr.PostalCode, lon, lat,
	).Scan(&addressID)
	if err != nil {
		return 0, err
	}

	status := "active"
	if p.Status != nil {
		status = *p.Status
	}
	var shopID int64
	err = tx.QueryRow(
		`INSERT INTO shops (name, status, phone, email, address_id)
		 VALUES ($1, $2::shop_status, $3, $4, $5)
		 RETURNING shop_id`,
		strings.TrimSpace(*p.Name), status, p.Phone, p.Email, addressID,
	).Scan(&shopID)
	if err != nil {
		return 0, err
	}
	if err := upsertSocials(tx, shopID, p); err != nil {
		return 0, err
	}
	return shopID, tx.Commit()
}

// socialPlatforms maps the flat payload fields onto socials table rows.
var socialPlatforms = []struct {
	platform string
	value    func(p *shopPayload) *string
}{
	{"website", func(p *shopPayload) *string { return p.Website }},
	{"instagram", func(p *shopPayload) *string { return p.InstagramURL }},
	{"facebook", func(p *shopPayload) *string { return p.FacebookURL }},
}

// upsertSocials applies the social link fields present in a payload: a value
// upserts the platform's row, an empty string removes it, nil leaves it alone.
func upsertSocials(tx *sql.Tx, shopID int64, p *shopPayload) error {
	for _, sp := range socialPlatforms {
		v := sp.value(p)
		if v == nil {
			continue
		}
		var err error
		if url := strings.TrimSpace(*v); url == "" {
			_, err = tx.Exec(`DELETE FROM socials WHERE shop_id = $1 AND platform = $2`, shopID, sp.platform)
		} else {
			_, err = tx.Exec(
				`INSERT INTO socials (shop_id, platform, url) VALUES ($1, $2, $3)
				 ON CONFLICT (shop_id, platform) DO UPDATE SET url = EXCLUDED.url, updated_at = NOW()`,
				shopID, sp.platform, url)
		}
		if err != nil {
			return fmt.Errorf("socials %s: %w", sp.platform, err)
		}
	}
	return nil
}

// updateShop handles PATCH /api/shops/{id}: fields present in the body are
// updated, everything else is left alone.
func (s *server) updateShop(w http.ResponseWriter, r *http.Request) {
	shopID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		badRequest(w, "invalid shop id")
		return
	}
	var p shopPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		badRequest(w, "invalid JSON: "+err.Error())
		return
	}
	if err := p.validate(false); err != nil {
		badRequest(w, err.Error())
		return
	}

	err = s.applyShopUpdate(shopID, &p)
	switch {
	case errors.Is(err, errShopNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	case errors.Is(err, errIncompleteAddress) || errors.Is(err, errNoAddress):
		badRequest(w, err.Error())
	case err != nil:
		s.internalError(w, err)
	default:
		s.respondWithShop(w, http.StatusOK, shopID)
	}
}

var (
	errShopNotFound = errors.New("shop not found")
	errNoAddress    = errors.New("shop has no address to locate; provide one")
)

// applyShopUpdate patches a shop from a validated payload: only non-nil
// fields change.
func (s *server) applyShopUpdate(shopID int64, p *shopPayload) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var addressID sql.NullInt64
	err = tx.QueryRow(`SELECT address_id FROM shops WHERE shop_id = $1 FOR UPDATE`, shopID).Scan(&addressID)
	if errors.Is(err, sql.ErrNoRows) {
		return errShopNotFound
	}
	if err != nil {
		return err
	}

	if p.Address != nil {
		addressID, err = s.applyAddress(tx, addressID, p.Address)
		if err != nil {
			return err
		}
	}

	// Explicit location wins; otherwise a changed address is re-geocoded so
	// the pin never points at the previous address.
	if p.Location != nil || p.Address != nil {
		if !addressID.Valid {
			return errNoAddress
		}
		var lat, lon sql.NullFloat64
		if p.Location != nil {
			lat = sql.NullFloat64{Float64: p.Location.Lat, Valid: true}
			lon = sql.NullFloat64{Float64: p.Location.Lon, Valid: true}
		} else {
			var a addressPayload
			err := tx.QueryRow(
				`SELECT street_number, street_name, city, COALESCE(borough, ''), postal_code
				 FROM addresses WHERE address_id = $1`, addressID.Int64,
			).Scan(&a.StreetNumber, &a.StreetName, &a.City, &a.Borough, &a.PostalCode)
			if err != nil {
				return err
			}
			if pt, ok, err := s.geo.Geocode(a.toNominatim()); err != nil {
				log.Printf("re-geocoding shop %d: %v", shopID, err)
			} else if ok {
				lat = sql.NullFloat64{Float64: pt.Lat, Valid: true}
				lon = sql.NullFloat64{Float64: pt.Lon, Valid: true}
			}
		}
		_, err = tx.Exec(
			`UPDATE addresses
			 SET location = ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, updated_at = NOW()
			 WHERE address_id = $3`,
			lon, lat, addressID.Int64)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(
		`UPDATE shops
		 SET name = COALESCE($1, name),
		     status = COALESCE($2::shop_status, status),
		     phone = COALESCE($3, phone),
		     email = COALESCE($4, email),
		     address_id = $5,
		     updated_at = NOW()
		 WHERE shop_id = $6`,
		p.Name, p.Status, p.Phone, p.Email, addressID, shopID)
	if err != nil {
		return err
	}
	if err := upsertSocials(tx, shopID, p); err != nil {
		return err
	}
	return tx.Commit()
}

var errIncompleteAddress = errors.New("shop has no address yet; street_number, street_name, city and postal_code are required")

// applyAddress patches the shop's existing address row, or inserts a new row
// when the shop has none (which requires the full address).
func (s *server) applyAddress(tx *sql.Tx, addressID sql.NullInt64, a *addressPayload) (sql.NullInt64, error) {
	if addressID.Valid {
		_, err := tx.Exec(
			`UPDATE addresses
			 SET street_number = COALESCE($1, street_number),
			     street_name = COALESCE($2, street_name),
			     city = COALESCE($3, city),
			     borough = COALESCE(NULLIF($4, ''), borough),
			     postal_code = COALESCE($5, postal_code),
			     updated_at = NOW()
			 WHERE address_id = $6`,
			a.StreetNumber, a.StreetName, a.City, a.Borough, a.PostalCode, addressID.Int64)
		return addressID, err
	}
	for _, v := range []*string{a.StreetNumber, a.StreetName, a.City, a.PostalCode} {
		if v == nil || strings.TrimSpace(*v) == "" {
			return addressID, errIncompleteAddress
		}
	}
	var id int64
	err := tx.QueryRow(
		`INSERT INTO addresses (street_number, street_name, city, borough, postal_code)
		 VALUES ($1, $2, $3, NULLIF($4, ''), $5)
		 RETURNING address_id`,
		*a.StreetNumber, *a.StreetName, *a.City, a.toNominatim().Borough, *a.PostalCode,
	).Scan(&id)
	return sql.NullInt64{Int64: id, Valid: err == nil}, err
}

const shopSelect = `
	SELECT shop_id, name, status, phone, email, website, instagram_url, facebook_url,
	       address_id, street_number, street_name, city, borough, postal_code,
	       ST_X(location::geometry), ST_Y(location::geometry), tags
	FROM shop_details`

// scanShopJSON reads one shopSelect row.
func scanShopJSON(scan func(...any) error) (shopJSON, error) {
	var (
		out      shopJSON
		addrID   sql.NullInt64
		street   sql.NullString
		number   sql.NullString
		city     sql.NullString
		borough  sql.NullString
		postal   sql.NullString
		lon, lat sql.NullFloat64
	)
	err := scan(&out.ID, &out.Name, &out.Status, &out.Phone, &out.Email, &out.Website,
		&out.InstagramURL, &out.FacebookURL,
		&addrID, &number, &street, &city, &borough, &postal, &lon, &lat, pq.Array(&out.Tags))
	if err != nil {
		return out, err
	}
	if addrID.Valid {
		out.Address = &addressJSON{
			StreetNumber: number.String,
			StreetName:   street.String,
			City:         city.String,
			PostalCode:   postal.String,
		}
		if borough.Valid {
			out.Address.Borough = &borough.String
		}
	}
	if lat.Valid && lon.Valid {
		out.Location = &locationPayload{Lat: lat.Float64, Lon: lon.Float64}
	}
	return out, nil
}

// getShop handles GET /api/shops/{id} (public).
func (s *server) getShop(w http.ResponseWriter, r *http.Request) {
	shopID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		badRequest(w, "invalid shop id")
		return
	}
	row := s.db.QueryRow(shopSelect+` WHERE shop_id = $1`, shopID)
	out, err := scanShopJSON(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "shop not found"})
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// listAllShops handles GET /api/shops/all (public): every shop, including the
// ones without coordinates that the GeoJSON route cannot show.
func (s *server) listAllShops(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(shopSelect + ` ORDER BY name`)
	if err != nil {
		s.internalError(w, err)
		return
	}
	defer rows.Close()
	out := []shopJSON{}
	for rows.Next() {
		shop, err := scanShopJSON(rows.Scan)
		if err != nil {
			s.internalError(w, err)
			return
		}
		out = append(out, shop)
	}
	if err := rows.Err(); err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// respondWithShop loads one shop and writes it as JSON.
func (s *server) respondWithShop(w http.ResponseWriter, code int, shopID int64) {
	row := s.db.QueryRow(shopSelect+` WHERE shop_id = $1`, shopID)
	out, err := scanShopJSON(row.Scan)
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, code, out)
}
