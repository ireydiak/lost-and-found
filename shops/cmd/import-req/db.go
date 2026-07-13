package main

import (
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/lib/pq"
)

// upsertRows writes shop rows into the database inside one transaction.
// (neq_id, neq_etab_id) is the natural key: existing shops and their addresses
// are updated in place, new ones are inserted.
func upsertRows(dsn string, rows []ShopRow) (inserted, updated int, err error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return 0, 0, err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	for _, r := range rows {
		var shopID int64
		var addressID sql.NullInt64
		err := tx.QueryRow(
			`SELECT shop_id, address_id FROM shops WHERE neq_id = $1 AND neq_etab_id = $2`,
			r.NEQ, r.EtabNo,
		).Scan(&shopID, &addressID)

		switch {
		case errors.Is(err, sql.ErrNoRows):
			var newAddrID int64
			if err := insertAddress(tx, r.Addr, &newAddrID); err != nil {
				return 0, 0, fmt.Errorf("shop %s (NEQ %s): %w", r.Name, r.NEQ, err)
			}
			err = tx.QueryRow(
				`INSERT INTO shops (name, status, neq_id, neq_etab_id, address_id)
				 VALUES ($1, $2::shop_status, $3, $4, $5)
				 RETURNING shop_id`,
				r.Name, r.Status, r.NEQ, r.EtabNo, newAddrID,
			).Scan(&shopID)
			if err != nil {
				return 0, 0, fmt.Errorf("inserting shop %s (NEQ %s): %w", r.Name, r.NEQ, err)
			}
			inserted++
		case err != nil:
			return 0, 0, err
		default:
			if addressID.Valid {
				_, err = tx.Exec(
					`UPDATE addresses
					 SET street_number = $1, street_name = $2, city = $3, borough = NULLIF($4, ''),
					     postal_code = $5, updated_at = NOW()
					 WHERE address_id = $6`,
					r.Addr.StreetNumber, r.Addr.StreetName, r.Addr.City, r.Addr.Borough, r.Addr.PostalCode, addressID.Int64,
				)
			} else {
				var newAddrID int64
				if err = insertAddress(tx, r.Addr, &newAddrID); err == nil {
					addressID = sql.NullInt64{Int64: newAddrID, Valid: true}
				}
			}
			if err != nil {
				return 0, 0, fmt.Errorf("updating address for shop %s (NEQ %s): %w", r.Name, r.NEQ, err)
			}
			_, err = tx.Exec(
				`UPDATE shops
				 SET name = $1, status = $2::shop_status, address_id = $3, updated_at = NOW()
				 WHERE shop_id = $4`,
				r.Name, r.Status, addressID.Int64, shopID,
			)
			if err != nil {
				return 0, 0, fmt.Errorf("updating shop %s (NEQ %s): %w", r.Name, r.NEQ, err)
			}
			updated++
		}

		for _, tag := range r.Tags {
			_, err = tx.Exec(
				`INSERT INTO shops_tags (shop_id, tag_id)
				 SELECT $1, tag_id FROM tags WHERE name = $2
				 ON CONFLICT DO NOTHING`,
				shopID, tag,
			)
			if err != nil {
				return 0, 0, fmt.Errorf("tagging shop %s with %s: %w", r.Name, tag, err)
			}
		}
	}

	return inserted, updated, tx.Commit()
}

func insertAddress(tx *sql.Tx, a Address, id *int64) error {
	return tx.QueryRow(
		`INSERT INTO addresses (street_number, street_name, city, borough, postal_code)
		 VALUES ($1, $2, $3, NULLIF($4, ''), $5)
		 RETURNING address_id`,
		a.StreetNumber, a.StreetName, a.City, a.Borough, a.PostalCode,
	).Scan(id)
}
