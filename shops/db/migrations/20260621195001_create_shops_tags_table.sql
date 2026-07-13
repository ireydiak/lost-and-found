-- +goose Up
CREATE TABLE IF NOT EXISTS shops_tags (
    shop_id BIGINT NOT NULL,
    tag_id BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_shop_id
        FOREIGN KEY (shop_id)
        REFERENCES shops(shop_id)
        ON DELETE CASCADE,

    CONSTRAINT fk_tag_id
        FOREIGN KEY (tag_id)
        REFERENCES tags(tag_id)
        ON DELETE RESTRICT,

    PRIMARY KEY (shop_id, tag_id)
);

-- +goose Down
DROP TABLE IF EXISTS shops_tags;
