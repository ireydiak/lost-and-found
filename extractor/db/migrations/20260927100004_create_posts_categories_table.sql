-- +goose Up
CREATE TABLE IF NOT EXISTS posts_categories (
    tag_id BIGINT NOT NULL,
    html_hash CHAR(64) NOT NULL,

    CONSTRAINT fk_tag_id
        FOREIGN KEY (tag_id)
        REFERENCES tags(tag_id)
        ON DELETE RESTRICT,

    CONSTRAINT fk_html_hash
        FOREIGN KEY (html_hash)
        REFERENCES posts_details(html_hash)
        ON DELETE CASCADE,

    PRIMARY KEY (tag_id, html_hash)
);

-- +goose Down
DROP TABLE IF EXISTS posts_categories;
