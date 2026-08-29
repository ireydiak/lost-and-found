-- +goose Up
-- C&L Cycle (NEQ 1166380767) operates a second location that was never
-- registered as a separate REQ establishment, so the import pipeline never
-- picked it up. Added as its own shop row, same as the existing location
-- (neq_id/neq_etab_id left unset since this isn't sourced from the
-- registry, matching how the BUM.bike community locations are recorded).
-- Coordinates geocoded via Nominatim from the street address.
INSERT INTO shops (name, status, street_number, street_name, city, postal_code, location, tags)
VALUES (
    'C&L Cycle', 'active', '978', 'Rue Rachel E', 'Montréal', 'H2J 2J3',
    ST_SetSRID(ST_MakePoint(-73.5748936, 45.5251683), 4326)::geography,
    ARRAY['bike-shop']
);

-- +goose Down
DELETE FROM shops WHERE name = 'C&L Cycle' AND street_number = '978' AND street_name = 'Rue Rachel E';
