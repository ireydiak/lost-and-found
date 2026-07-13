-- +goose Up
-- One row per link so new platforms are inserts, not migrations.
CREATE TABLE IF NOT EXISTS socials (
    social_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    shop_id BIGINT NOT NULL REFERENCES shops(shop_id) ON DELETE CASCADE,
    platform VARCHAR(30) NOT NULL,
    url VARCHAR(500) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT socials_shop_platform_uniq UNIQUE (shop_id, platform)
);

INSERT INTO socials (shop_id, platform, url)
SELECT shop_id, 'website', website FROM shops WHERE COALESCE(website, '') <> ''
UNION ALL
SELECT shop_id, 'instagram', instagram_url FROM shops WHERE COALESCE(instagram_url, '') <> ''
UNION ALL
SELECT shop_id, 'facebook', facebook_url FROM shops WHERE COALESCE(facebook_url, '') <> '';

ALTER TABLE shops
    DROP COLUMN website,
    DROP COLUMN instagram_url,
    DROP COLUMN facebook_url;

-- Flat read model: shops + address + socials pivoted back into columns.
CREATE VIEW shop_details AS
SELECT s.shop_id, s.name, s.status, s.phone, s.email, s.neq_id, s.neq_etab_id,
       a.address_id, a.street_number, a.street_name, a.city, a.borough, a.postal_code, a.location,
       soc.website, soc.instagram_url, soc.facebook_url
FROM shops s
LEFT JOIN addresses a ON a.address_id = s.address_id
LEFT JOIN (
    SELECT shop_id,
           max(url) FILTER (WHERE platform = 'website') AS website,
           max(url) FILTER (WHERE platform = 'instagram') AS instagram_url,
           max(url) FILTER (WHERE platform = 'facebook') AS facebook_url
    FROM socials
    GROUP BY shop_id
) soc ON soc.shop_id = s.shop_id;

-- +goose Down
DROP VIEW IF EXISTS shop_details;

ALTER TABLE shops
    ADD COLUMN website VARCHAR(500),
    ADD COLUMN instagram_url VARCHAR(500),
    ADD COLUMN facebook_url VARCHAR(500);

UPDATE shops s SET
    website = (SELECT url FROM socials WHERE shop_id = s.shop_id AND platform = 'website'),
    instagram_url = (SELECT url FROM socials WHERE shop_id = s.shop_id AND platform = 'instagram'),
    facebook_url = (SELECT url FROM socials WHERE shop_id = s.shop_id AND platform = 'facebook');

DROP TABLE IF EXISTS socials;
