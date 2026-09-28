-- +goose Up
CREATE TABLE IF NOT EXISTS posts_meta (
    html_hash CHAR(64) PRIMARY KEY,
    filename VARCHAR(500) NOT NULL,
    sourced_at TIMESTAMPTZ NOT NULL,
    facebook_id VARCHAR(50)
);

-- Partial index: facebook_id is only ever looked up when present (see
-- docs/DATABASE.md Deduplication) -- most rows won't have one, so indexing
-- only the non-null subset keeps the index smaller without losing anything
-- an equality lookup would use anyway.
CREATE INDEX IF NOT EXISTS idx_posts_meta_facebook_id ON posts_meta(facebook_id) WHERE facebook_id IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS posts_meta;
