-- +goose Up
CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TYPE shop_status AS ENUM ('active', 'inactive', 'closed');

-- +goose Down
DROP TYPE IF EXISTS shop_status;
