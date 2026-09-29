-- +goose Up
CREATE TYPE post_asset_type AS ENUM ('picture', 'video');

CREATE TABLE IF NOT EXISTS posts_assets (
    asset_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    html_hash CHAR(64) NOT NULL,
    -- TEXT, not VARCHAR(n): Facebook's CDN URLs carry long, unpredictable
    -- query strings (signing tokens, tracking params) -- no safe fixed cap
    -- observed in practice.
    url TEXT NOT NULL,
    asset_type post_asset_type NOT NULL,

    CONSTRAINT fk_html_hash
        FOREIGN KEY (html_hash)
        REFERENCES posts_details(html_hash)
        ON DELETE CASCADE,

    -- Guards against the same extraction run inserting the same asset twice;
    -- does not (and cannot) dedupe the same real photo across separate
    -- scrapes, since Facebook's CDN URLs differ per request even for an
    -- identical image.
    CONSTRAINT uq_html_hash_url
        UNIQUE (html_hash, url)
);

CREATE INDEX IF NOT EXISTS idx_posts_assets_html_hash ON posts_assets(html_hash);

-- +goose Down
DROP TABLE IF EXISTS posts_assets;
DROP TYPE IF EXISTS post_asset_type;
