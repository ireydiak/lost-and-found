-- +goose Up
-- Collapses shops + addresses + socials + tags/shops_tags into a single
-- shops table. addresses and socials were always a strict 1:1 (every shop
-- has at most one address/one link per platform); tags become a plain
-- array since no shop has ever needed more than one and nothing in the app
-- reads the tags catalog's display_name.
ALTER TABLE shops
    ADD COLUMN website VARCHAR(500),
    ADD COLUMN instagram_url VARCHAR(500),
    ADD COLUMN facebook_url VARCHAR(500),
    ADD COLUMN street_number VARCHAR(10),
    ADD COLUMN street_name VARCHAR(255),
    ADD COLUMN city VARCHAR(255),
    ADD COLUMN borough VARCHAR(255),
    ADD COLUMN postal_code VARCHAR(7),
    ADD COLUMN location GEOGRAPHY(Point, 4326),
    ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}';

UPDATE shops s
SET street_number = a.street_number,
    street_name = a.street_name,
    city = a.city,
    borough = a.borough,
    postal_code = a.postal_code,
    location = a.location
FROM addresses a
WHERE a.address_id = s.address_id;

UPDATE shops s
SET website = soc.website,
    instagram_url = soc.instagram_url,
    facebook_url = soc.facebook_url
FROM (
    SELECT shop_id,
           max(url) FILTER (WHERE platform = 'website') AS website,
           max(url) FILTER (WHERE platform = 'instagram') AS instagram_url,
           max(url) FILTER (WHERE platform = 'facebook') AS facebook_url
    FROM socials
    GROUP BY shop_id
) soc
WHERE soc.shop_id = s.shop_id;

UPDATE shops s
SET tags = tg.tags
FROM (
    SELECT st.shop_id, array_agg(t.name ORDER BY t.name) AS tags
    FROM shops_tags st
    JOIN tags t ON t.tag_id = st.tag_id
    GROUP BY st.shop_id
) tg
WHERE tg.shop_id = s.shop_id;

DROP VIEW IF EXISTS shop_details;
DROP TABLE shops_tags;
DROP TABLE tags;
DROP TABLE socials;

ALTER TABLE shops DROP COLUMN address_id;

DROP TABLE addresses;

-- +goose Down
ALTER TABLE shops ADD COLUMN address_id BIGINT;

CREATE TABLE addresses (
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

ALTER TABLE shops
    ADD CONSTRAINT fk_address_id
        FOREIGN KEY (address_id)
        REFERENCES addresses(address_id)
        ON DELETE SET NULL;

-- One address row per shop that has one; addresses were never shared, so a
-- straight per-shop loop reproduces the original 1:1 layout exactly.
-- +goose StatementBegin
DO $$
DECLARE
    r RECORD;
    new_id BIGINT;
BEGIN
    FOR r IN
        SELECT shop_id, street_number, street_name, city, borough, postal_code, location
        FROM shops WHERE street_number IS NOT NULL
    LOOP
        INSERT INTO addresses (street_number, street_name, city, borough, postal_code, location)
        VALUES (r.street_number, r.street_name, r.city, r.borough, r.postal_code, r.location)
        RETURNING address_id INTO new_id;

        UPDATE shops SET address_id = new_id WHERE shop_id = r.shop_id;
    END LOOP;
END $$;
-- +goose StatementEnd

CREATE TABLE socials (
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

CREATE TABLE tags (
    tag_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    display_name VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO tags (name, display_name) VALUES
    ('bike-shop', 'Bike Shop'),
    ('used-bikes', 'Used Bikes'),
    ('repair', 'Repair'),
    ('rental', 'Rental'),
    ('accessories', 'Accessories'),
    ('custom-builds', 'Custom Builds'),
    ('sporting-goods', 'Sporting Goods'),
    ('unknown', 'Unknown'),
    ('pawn-shop', 'Pawn Shop'),
    ('non-profit', 'Non-Profit');

CREATE TABLE shops_tags (
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

INSERT INTO shops_tags (shop_id, tag_id)
SELECT s.shop_id, t.tag_id
FROM shops s
CROSS JOIN LATERAL unnest(s.tags) AS tag_name
JOIN tags t ON t.name = tag_name;

CREATE VIEW shop_details AS
SELECT s.shop_id, s.name, s.status, s.phone, s.email, s.neq_id, s.neq_etab_id,
       a.address_id, a.street_number, a.street_name, a.city, a.borough, a.postal_code, a.location,
       soc.website, soc.instagram_url, soc.facebook_url,
       COALESCE(tg.tags, ARRAY[]::text[]) AS tags
FROM shops s
LEFT JOIN addresses a ON a.address_id = s.address_id
LEFT JOIN (
    SELECT shop_id,
           max(url) FILTER (WHERE platform = 'website') AS website,
           max(url) FILTER (WHERE platform = 'instagram') AS instagram_url,
           max(url) FILTER (WHERE platform = 'facebook') AS facebook_url
    FROM socials
    GROUP BY shop_id
) soc ON soc.shop_id = s.shop_id
LEFT JOIN (
    SELECT st.shop_id, array_agg(t.name ORDER BY t.name) AS tags
    FROM shops_tags st
    JOIN tags t ON t.tag_id = st.tag_id
    GROUP BY st.shop_id
) tg ON tg.shop_id = s.shop_id;

ALTER TABLE shops
    DROP COLUMN website,
    DROP COLUMN instagram_url,
    DROP COLUMN facebook_url,
    DROP COLUMN street_number,
    DROP COLUMN street_name,
    DROP COLUMN city,
    DROP COLUMN borough,
    DROP COLUMN postal_code,
    DROP COLUMN location,
    DROP COLUMN tags;
