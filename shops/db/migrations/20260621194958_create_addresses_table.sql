-- +goose Up
CREATE TABLE IF NOT EXISTS addresses (
    address_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    street_number VARCHAR(10) NOT NULL,
    street_name VARCHAR(255) NOT NULL,
    city VARCHAR(255) NOT NULL,
    borough VARCHAR(255),
    province VARCHAR(10) NOT NULL DEFAULT 'QC',
    postal_code VARCHAR(7) NOT NULL,
    location GEOGRAPHY(Point, 4326),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS addresses;
