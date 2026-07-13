-- +goose Up
-- neq_etab_id is the REQ establishment sequence number (NO_SUF_ETAB).
-- 0 means the shop comes from the enterprise's domicile address (no establishment row).
ALTER TABLE shops ADD COLUMN neq_etab_id INTEGER NOT NULL DEFAULT 0;

CREATE UNIQUE INDEX IF NOT EXISTS uq_shops_neq_id_etab ON shops(neq_id, neq_etab_id);

-- +goose Down
DROP INDEX IF EXISTS uq_shops_neq_id_etab;
ALTER TABLE shops DROP COLUMN IF EXISTS neq_etab_id;
