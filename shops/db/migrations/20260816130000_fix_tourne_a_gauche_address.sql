-- +goose Up
-- "Tourne à Gauche" runs out of Pacemaker (lespacemaker.com), whose contact
-- page gives the real civic number — replaces the 's/n' placeholder used in
-- 20260816120001_seed_bum_bike_shops.sql (reverse-geocoding had no house
-- number for that point).
UPDATE addresses SET street_number = '2875'
WHERE address_id = (SELECT address_id FROM shops WHERE name = 'Tourne à Gauche');

-- +goose Down
UPDATE addresses SET street_number = 's/n'
WHERE address_id = (SELECT address_id FROM shops WHERE name = 'Tourne à Gauche');
