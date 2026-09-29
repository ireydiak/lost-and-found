-- +goose Up
CREATE TABLE IF NOT EXISTS posts_details (
    html_hash CHAR(64) PRIMARY KEY,
    author VARCHAR(255) NOT NULL,
    title VARCHAR(500),
    description TEXT,
    date TIMESTAMPTZ NOT NULL,
    dedup_key VARCHAR(255) NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL,

    CONSTRAINT fk_html_hash
        FOREIGN KEY (html_hash)
        REFERENCES posts_meta(html_hash)
        ON DELETE CASCADE
);

-- Used by the fallback dedup lookup (see docs/DATABASE.md Deduplication)
-- when a candidate row has no facebook_id to match on.
CREATE INDEX IF NOT EXISTS idx_posts_details_dedup_key ON posts_details(dedup_key);

-- +goose Down
DROP TABLE IF EXISTS posts_details;
