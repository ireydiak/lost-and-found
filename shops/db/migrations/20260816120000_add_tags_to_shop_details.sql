-- +goose Up
CREATE OR REPLACE VIEW shop_details AS
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

-- +goose Down
CREATE OR REPLACE VIEW shop_details AS
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
