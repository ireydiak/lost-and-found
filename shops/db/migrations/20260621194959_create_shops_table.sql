-- +goose Up
CREATE TABLE IF NOT EXISTS shops (
    shop_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    status shop_status NOT NULL DEFAULT 'active',
    phone VARCHAR(20),
    email VARCHAR(255),
    website VARCHAR(500),
    instagram_url VARCHAR(500),
    facebook_url VARCHAR(500),
    neq_id VARCHAR(10),
    address_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_address_id
        FOREIGN KEY (address_id)
        REFERENCES addresses(address_id)
        ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_shops_neq_id ON shops(neq_id);

-- +goose Down
DROP TABLE IF EXISTS shops;
