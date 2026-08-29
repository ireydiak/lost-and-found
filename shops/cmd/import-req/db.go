package main

import (
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/lib/pq"
)

// upsertRows writes shop rows into the database inside one transaction.
// (neq_id, neq_etab_id) is the natural key: existing shops are updated in
// place, new ones are inserted.
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
		err := tx.QueryRow(
			`SELECT shop_id FROM shops WHERE neq_id = $1 AND neq_etab_id = $2`,
			r.NEQ, r.EtabNo,
		).Scan(&shopID)

		switch {
		case errors.Is(err, sql.ErrNoRows):
			err = tx.QueryRow(
				`INSERT INTO shops (name, status, neq_id, neq_etab_id,
				                     street_number, street_name, city, borough, postal_code)
				 VALUES ($1, $2::shop_status, $3, $4, $5, $6, $7, NULLIF($8, ''), $9)
				 RETURNING shop_id`,
				r.Name, r.Status, r.NEQ, r.EtabNo,
				r.Addr.StreetNumber, r.Addr.StreetName, r.Addr.City, r.Addr.Borough, r.Addr.PostalCode,
			).Scan(&shopID)
			if err != nil {
				return 0, 0, fmt.Errorf("inserting shop %s (NEQ %s): %w", r.Name, r.NEQ, err)
			}
			inserted++
		case err != nil:
			return 0, 0, err
		default:
			_, err = tx.Exec(
				`UPDATE shops
				 SET name = $1, status = $2::shop_status,
				     street_number = $3, street_name = $4, city = $5, borough = NULLIF($6, ''), postal_code = $7,
				     updated_at = NOW()
				 WHERE shop_id = $8`,
				r.Name, r.Status, r.Addr.StreetNumber, r.Addr.StreetName, r.Addr.City, r.Addr.Borough, r.Addr.PostalCode, shopID,
			)
			if err != nil {
				return 0, 0, fmt.Errorf("updating shop %s (NEQ %s): %w", r.Name, r.NEQ, err)
			}
			updated++
		}

		for _, tag := range r.Tags {
			_, err = tx.Exec(
				`UPDATE shops SET tags = array_append(tags, $1), updated_at = NOW()
				 WHERE shop_id = $2 AND NOT ($1 = ANY(tags))`,
				tag, shopID,
			)
			if err != nil {
				return 0, 0, fmt.Errorf("tagging shop %s with %s: %w", r.Name, tag, err)
			}
		}
	}

	return inserted, updated, tx.Commit()
}
