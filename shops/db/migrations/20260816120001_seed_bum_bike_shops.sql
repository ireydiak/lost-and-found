-- +goose Up
-- Non-profit bike shops/co-ops sourced from https://bum.bike/_data/ateliers.json
-- (Bikes Unite Montreal). That feed has coordinates but no street addresses,
-- so each address below was reverse-geocoded from the known point via
-- Nominatim. location is set directly from the source coordinates, not
-- re-derived by forward-geocoding the reverse-geocoded address.
--
-- 4 locations (Atelier Culture Vélo, Tourne à Gauche, Mile End Bike Garage,
-- Têtes de rayon) sit inside a park, campus, or community building with no
-- civic number; street_number is 's/n' (sans numéro) for those — the pin
-- location is still precise, only the house number is unknown.
--
-- The 'non-profit' tag is inserted here rather than as a db/seeds file:
-- cmd/migrate runs all of db/migrations before db/seeds, so a migration
-- can't rely on seed data existing yet.
INSERT INTO tags (name, display_name) VALUES ('non-profit', 'Non-Profit')
ON CONFLICT (name) DO NOTHING;

WITH addr AS (
  INSERT INTO addresses (street_number, street_name, city, borough, postal_code, location)
  VALUES ('3511', 'Rue Peel', 'Montréal', 'Ville-Marie', 'H3A 3T6',
          ST_SetSRID(ST_MakePoint(-73.578281077645997, 45.503584453359089), 4326)::geography)
  RETURNING address_id
), shop AS (
  INSERT INTO shops (name, status, address_id)
  SELECT 'The Flat Bike Collective', 'active', address_id FROM addr
  RETURNING shop_id
), tagging AS (
  INSERT INTO shops_tags (shop_id, tag_id)
  SELECT shop.shop_id, tags.tag_id FROM shop, tags WHERE tags.name = 'non-profit'
  RETURNING shop_id
)
INSERT INTO socials (shop_id, platform, url)
SELECT shop_id, 'website', 'https://theflat.wordpress.com/' FROM tagging;

WITH addr AS (
  INSERT INTO addresses (street_number, street_name, city, borough, postal_code, location)
  VALUES ('2700', 'Chemin de l''Est', 'Montréal', 'Côte-des-Neiges–Notre-Dame-de-Grâce', 'H3T 1J4',
          ST_SetSRID(ST_MakePoint(-73.61438520986488, 45.50437737179211), 4326)::geography)
  RETURNING address_id
), shop AS (
  INSERT INTO shops (name, status, address_id)
  SELECT 'Biciklo', 'active', address_id FROM addr
  RETURNING shop_id
), tagging AS (
  INSERT INTO shops_tags (shop_id, tag_id)
  SELECT shop.shop_id, tags.tag_id FROM shop, tags WHERE tags.name = 'non-profit'
  RETURNING shop_id
)
INSERT INTO socials (shop_id, platform, url)
SELECT shop_id, 'website', 'https://polyfab.polymtl.ca/biciklo/inscription/' FROM tagging;

WITH addr AS (
  INSERT INTO addresses (street_number, street_name, city, borough, postal_code, location)
  VALUES ('3200', 'Chemin de la Côte-Sainte-Catherine', 'Montréal', 'Côte-des-Neiges–Notre-Dame-de-Grâce', 'H3T 1C1',
          ST_SetSRID(ST_MakePoint(-73.62334229807892, 45.50119635993943), 4326)::geography)
  RETURNING address_id
), shop AS (
  INSERT INTO shops (name, status, address_id)
  SELECT 'Coop Bécik', 'active', address_id FROM addr
  RETURNING shop_id
), tagging AS (
  INSERT INTO shops_tags (shop_id, tag_id)
  SELECT shop.shop_id, tags.tag_id FROM shop, tags WHERE tags.name = 'non-profit'
  RETURNING shop_id
)
INSERT INTO socials (shop_id, platform, url)
SELECT shop_id, 'instagram', 'https://www.instagram.com/coopbecik_brebeuf/' FROM tagging;

WITH addr AS (
  INSERT INTO addresses (street_number, street_name, city, borough, postal_code, location)
  VALUES ('385', 'Rue Sherbrooke Est', 'Montréal', 'Ville-Marie', 'H2X 1E6',
          ST_SetSRID(ST_MakePoint(-73.56790402, 45.51688699), 4326)::geography)
  RETURNING address_id
), shop AS (
  INSERT INTO shops (name, status, address_id)
  SELECT 'Nid de Poule', 'active', address_id FROM addr
  RETURNING shop_id
)
INSERT INTO shops_tags (shop_id, tag_id)
SELECT shop.shop_id, tags.tag_id FROM shop, tags WHERE tags.name = 'non-profit';

WITH addr AS (
  INSERT INTO addresses (street_number, street_name, city, borough, postal_code, location)
  VALUES ('1100', 'Rue Notre-Dame Ouest', 'Montréal', 'Le Sud-Ouest', 'H3C 6M8',
          ST_SetSRID(ST_MakePoint(-73.56253339, 45.49496663), 4326)::geography)
  RETURNING address_id
), shop AS (
  INSERT INTO shops (name, status, address_id)
  SELECT 'Le C.R.A.B.E.', 'active', address_id FROM addr
  RETURNING shop_id
), tagging AS (
  INSERT INTO shops_tags (shop_id, tag_id)
  SELECT shop.shop_id, tags.tag_id FROM shop, tags WHERE tags.name = 'non-profit'
  RETURNING shop_id
)
INSERT INTO socials (shop_id, platform, url)
SELECT shop_id, 'website', 'http://crabe.etsmtl.ca/' FROM tagging;

WITH addr AS (
  INSERT INTO addresses (street_number, street_name, city, borough, postal_code, location)
  VALUES ('1900', 'Rue Sainte-Madeleine', 'Montréal', 'Le Sud-Ouest', 'H3K 2A4',
          ST_SetSRID(ST_MakePoint(-73.55248265, 45.48111285), 4326)::geography)
  RETURNING address_id
), shop AS (
  INSERT INTO shops (name, status, address_id)
  SELECT 'Cycle 7', 'active', address_id FROM addr
  RETURNING shop_id
), tagging AS (
  INSERT INTO shops_tags (shop_id, tag_id)
  SELECT shop.shop_id, tags.tag_id FROM shop, tags WHERE tags.name = 'non-profit'
  RETURNING shop_id
)
INSERT INTO socials (shop_id, platform, url)
SELECT shop_id, 'website', 'https://www.cycle7.ca/heures' FROM tagging;

WITH addr AS (
  INSERT INTO addresses (street_number, street_name, city, borough, postal_code, location)
  VALUES ('1455', 'Boulevard De Maisonneuve Ouest', 'Montréal', 'Ville-Marie', 'H3G 1M8',
          ST_SetSRID(ST_MakePoint(-73.57941043, 45.49761647), 4326)::geography)
  RETURNING address_id
), shop AS (
  INSERT INTO shops (name, status, address_id)
  SELECT 'Right to Move / La Voie Libre', 'active', address_id FROM addr
  RETURNING shop_id
), tagging AS (
  INSERT INTO shops_tags (shop_id, tag_id)
  SELECT shop.shop_id, tags.tag_id FROM shop, tags WHERE tags.name = 'non-profit'
  RETURNING shop_id
)
INSERT INTO socials (shop_id, platform, url)
SELECT shop_id, 'website', 'https://rtm-lvl.org/fr/heures/' FROM tagging;

WITH addr AS (
  INSERT INTO addresses (street_number, street_name, city, borough, postal_code, location)
  VALUES ('s/n', 'Rue Jarry Ouest', 'Montréal', 'Villeray–Saint-Michel–Parc-Extension', 'H2P 1S6',
          ST_SetSRID(ST_MakePoint(-73.63084125, 45.53506429), 4326)::geography)
  RETURNING address_id
), shop AS (
  INSERT INTO shops (name, status, address_id)
  SELECT 'Atelier Culture Vélo', 'active', address_id FROM addr
  RETURNING shop_id
), tagging AS (
  INSERT INTO shops_tags (shop_id, tag_id)
  SELECT shop.shop_id, tags.tag_id FROM shop, tags WHERE tags.name = 'non-profit'
  RETURNING shop_id
)
INSERT INTO socials (shop_id, platform, url)
SELECT shop_id, 'facebook', 'https://www.facebook.com/atelierculturevelo/' FROM tagging;

WITH addr AS (
  INSERT INTO addresses (street_number, street_name, city, borough, postal_code, location)
  VALUES ('200', 'Rue Sherbrooke Ouest', 'Montréal', 'Le Plateau-Mont-Royal', 'H2X 3P2',
          ST_SetSRID(ST_MakePoint(-73.57001538, 45.50981), 4326)::geography)
  RETURNING address_id
), shop AS (
  INSERT INTO shops (name, status, address_id)
  SELECT 'BQAM-E', 'active', address_id FROM addr
  RETURNING shop_id
), tagging AS (
  INSERT INTO shops_tags (shop_id, tag_id)
  SELECT shop.shop_id, tags.tag_id FROM shop, tags WHERE tags.name = 'non-profit'
  RETURNING shop_id
)
INSERT INTO socials (shop_id, platform, url)
SELECT shop_id, 'website', 'https://bqam-e.org/' FROM tagging;

WITH addr AS (
  INSERT INTO addresses (street_number, street_name, city, borough, postal_code, location)
  VALUES ('111', 'Rue Roy Est', 'Montréal', 'Le Plateau-Mont-Royal', 'H2W 1M2',
          ST_SetSRID(ST_MakePoint(-73.57521853, 45.51668767), 4326)::geography)
  RETURNING address_id
), shop AS (
  INSERT INTO shops (name, status, address_id)
  SELECT 'Santrovélo', 'active', address_id FROM addr
  RETURNING shop_id
), tagging AS (
  INSERT INTO shops_tags (shop_id, tag_id)
  SELECT shop.shop_id, tags.tag_id FROM shop, tags WHERE tags.name = 'non-profit'
  RETURNING shop_id
)
INSERT INTO socials (shop_id, platform, url)
SELECT shop_id, 'website', 'https://santrovelo.square.site/' FROM tagging;

WITH addr AS (
  INSERT INTO addresses (street_number, street_name, city, borough, postal_code, location)
  VALUES ('6450', 'Avenue Christophe-Colomb', 'Montréal', 'Rosemont–La Petite-Patrie', 'H2S 2G7',
          ST_SetSRID(ST_MakePoint(-73.60156388, 45.53746646), 4326)::geography)
  RETURNING address_id
), shop AS (
  INSERT INTO shops (name, status, address_id)
  SELECT 'La Remise', 'active', address_id FROM addr
  RETURNING shop_id
), tagging AS (
  INSERT INTO shops_tags (shop_id, tag_id)
  SELECT shop.shop_id, tags.tag_id FROM shop, tags WHERE tags.name = 'non-profit'
  RETURNING shop_id
)
INSERT INTO socials (shop_id, platform, url)
SELECT shop_id, 'website', 'https://laremise.ca/les-ateliers/atelier-velo/' FROM tagging;

WITH addr AS (
  INSERT INTO addresses (street_number, street_name, city, borough, postal_code, location)
  VALUES ('7525', 'Rue François-Perrault', 'Montréal', 'Villeray–Saint-Michel–Parc-Extension', 'H2A 3L6',
          ST_SetSRID(ST_MakePoint(-73.60078297, 45.56242496), 4326)::geography)
  RETURNING address_id
), shop AS (
  INSERT INTO shops (name, status, address_id)
  SELECT 'La Grande Roue', 'active', address_id FROM addr
  RETURNING shop_id
), tagging AS (
  INSERT INTO shops_tags (shop_id, tag_id)
  SELECT shop.shop_id, tags.tag_id FROM shop, tags WHERE tags.name = 'non-profit'
  RETURNING shop_id
)
INSERT INTO socials (shop_id, platform, url)
SELECT shop_id, 'website', 'https://www.lcsm.qc.ca/permanences' FROM tagging;

WITH addr AS (
  INSERT INTO addresses (street_number, street_name, city, borough, postal_code, location)
  VALUES ('s/n', 'Rue Hochelaga', 'Montréal', 'Ville-Marie', 'H2K 1K7',
          ST_SetSRID(ST_MakePoint(-73.55663054, 45.53922831), 4326)::geography)
  RETURNING address_id
), shop AS (
  INSERT INTO shops (name, status, address_id)
  SELECT 'Tourne à Gauche', 'active', address_id FROM addr
  RETURNING shop_id
), tagging AS (
  INSERT INTO shops_tags (shop_id, tag_id)
  SELECT shop.shop_id, tags.tag_id FROM shop, tags WHERE tags.name = 'non-profit'
  RETURNING shop_id
)
INSERT INTO socials (shop_id, platform, url)
SELECT shop_id, 'website', 'https://www.lespacemaker.com/fr/calendriers/' FROM tagging;

WITH addr AS (
  INSERT INTO addresses (street_number, street_name, city, borough, postal_code, location)
  VALUES ('2365', 'Rue Grand Trunk', 'Montréal', 'Le Sud-Ouest', 'H3K 1M8',
          ST_SetSRID(ST_MakePoint(-73.5654363, 45.47892323), 4326)::geography)
  RETURNING address_id
), shop AS (
  INSERT INTO shops (name, status, address_id)
  SELECT 'Bill''s Bike Shop', 'active', address_id FROM addr
  RETURNING shop_id
), tagging AS (
  INSERT INTO shops_tags (shop_id, tag_id)
  SELECT shop.shop_id, tags.tag_id FROM shop, tags WHERE tags.name = 'non-profit'
  RETURNING shop_id
)
INSERT INTO socials (shop_id, platform, url)
SELECT shop_id, 'website', 'https://saintcolumbahouse.org/program/bills-community-bike-shop/' FROM tagging;

WITH addr AS (
  INSERT INTO addresses (street_number, street_name, city, borough, postal_code, location)
  VALUES ('7141', 'Rue Sherbrooke Ouest', 'Montréal', 'Côte-des-Neiges–Notre-Dame-de-Grâce', 'H4B 1R6',
          ST_SetSRID(ST_MakePoint(-73.6396048, 45.45947092), 4326)::geography)
  RETURNING address_id
), shop AS (
  INSERT INTO shops (name, status, address_id)
  SELECT 'Le Petit Vélo Rouge', 'active', address_id FROM addr
  RETURNING shop_id
), tagging AS (
  INSERT INTO shops_tags (shop_id, tag_id)
  SELECT shop.shop_id, tags.tag_id FROM shop, tags WHERE tags.name = 'non-profit'
  RETURNING shop_id
)
INSERT INTO socials (shop_id, platform, url)
SELECT shop_id, 'website', 'https://petitvelorouge.wordpress.com/' FROM tagging;

WITH addr AS (
  INSERT INTO addresses (street_number, street_name, city, borough, postal_code, location)
  VALUES ('s/n', 'Avenue Van Horne', 'Montréal', 'Le Plateau-Mont-Royal', 'H2T 3A3',
          ST_SetSRID(ST_MakePoint(-73.60789406, 45.52761758), 4326)::geography)
  RETURNING address_id
), shop AS (
  INSERT INTO shops (name, status, address_id)
  SELECT 'Mile End Bike Garage', 'active', address_id FROM addr
  RETURNING shop_id
), tagging AS (
  INSERT INTO shops_tags (shop_id, tag_id)
  SELECT shop.shop_id, tags.tag_id FROM shop, tags WHERE tags.name = 'non-profit'
  RETURNING shop_id
)
INSERT INTO socials (shop_id, platform, url)
SELECT shop_id, 'facebook', 'https://www.facebook.com/MileEndBikeGarage/' FROM tagging;

WITH addr AS (
  INSERT INTO addresses (street_number, street_name, city, borough, postal_code, location)
  VALUES ('1800', 'Rue la Fontaine', 'Montréal', 'Mercier–Hochelaga-Maisonneuve', 'H1V 2P8',
          ST_SetSRID(ST_MakePoint(-73.53814834, 45.55187797), 4326)::geography)
  RETURNING address_id
), shop AS (
  INSERT INTO shops (name, status, address_id)
  SELECT 'B-Shoppe', 'active', address_id FROM addr
  RETURNING shop_id
), tagging AS (
  INSERT INTO shops_tags (shop_id, tag_id)
  SELECT shop.shop_id, tags.tag_id FROM shop, tags WHERE tags.name = 'non-profit'
  RETURNING shop_id
)
INSERT INTO socials (shop_id, platform, url)
SELECT shop_id, 'facebook', 'https://www.facebook.com/bchoppe/' FROM tagging;

WITH addr AS (
  INSERT INTO addresses (street_number, street_name, city, borough, postal_code, location)
  VALUES ('s/n', 'Rue Lajeunesse', 'Montréal', 'Ahuntsic-Cartierville', 'H2M 2E5',
          ST_SetSRID(ST_MakePoint(-73.647643230015063, 45.549374529659914), 4326)::geography)
  RETURNING address_id
), shop AS (
  INSERT INTO shops (name, status, address_id)
  SELECT 'Têtes de rayon', 'active', address_id FROM addr
  RETURNING shop_id
), tagging AS (
  INSERT INTO shops_tags (shop_id, tag_id)
  SELECT shop.shop_id, tags.tag_id FROM shop, tags WHERE tags.name = 'non-profit'
  RETURNING shop_id
)
INSERT INTO socials (shop_id, platform, url)
SELECT shop_id, 'website', 'https://velo-edpahuntsic.square.site/' FROM tagging;

WITH addr AS (
  INSERT INTO addresses (street_number, street_name, city, borough, postal_code, location)
  VALUES ('3830', 'Boulevard Henri-Bourassa Est', 'Montréal', 'Montréal-Nord', 'H1H 5M3',
          ST_SetSRID(ST_MakePoint(-73.64487012321497, 45.59244652222314), 4326)::geography)
  RETURNING address_id
), shop AS (
  INSERT INTO shops (name, status, address_id)
  SELECT 'Atelier Vélo Montréal-Nord', 'active', address_id FROM addr
  RETURNING shop_id
), tagging AS (
  INSERT INTO shops_tags (shop_id, tag_id)
  SELECT shop.shop_id, tags.tag_id FROM shop, tags WHERE tags.name = 'non-profit'
  RETURNING shop_id
)
INSERT INTO socials (shop_id, platform, url)
SELECT shop_id, 'website', 'https://atelier-velo-montrealnord.square.site/' FROM tagging;

WITH addr AS (
  INSERT INTO addresses (street_number, street_name, city, borough, postal_code, location)
  VALUES ('1505', 'Rue Cardinal', 'Montréal', 'Saint-Laurent', 'H4L 3G3',
          ST_SetSRID(ST_MakePoint(-73.69110288728614, 45.51800713895457), 4326)::geography)
  RETURNING address_id
), shop AS (
  INSERT INTO shops (name, status, address_id)
  SELECT 'Vélogik', 'active', address_id FROM addr
  RETURNING shop_id
), tagging AS (
  INSERT INTO shops_tags (shop_id, tag_id)
  SELECT shop.shop_id, tags.tag_id FROM shop, tags WHERE tags.name = 'non-profit'
  RETURNING shop_id
)
INSERT INTO socials (shop_id, platform, url)
SELECT shop_id, 'facebook', 'https://www.facebook.com/velogik/' FROM tagging;

-- +goose Down
DELETE FROM addresses WHERE address_id IN (
    SELECT address_id FROM shops WHERE name IN (
        'The Flat Bike Collective', 'Biciklo', 'Coop Bécik', 'Nid de Poule', 'Le C.R.A.B.E.',
        'Cycle 7', 'Right to Move / La Voie Libre', 'Atelier Culture Vélo', 'BQAM-E', 'Santrovélo',
        'La Remise', 'La Grande Roue', 'Tourne à Gauche', 'Bill''s Bike Shop', 'Le Petit Vélo Rouge',
        'Mile End Bike Garage', 'B-Shoppe', 'Têtes de rayon', 'Atelier Vélo Montréal-Nord', 'Vélogik'
    )
);
DELETE FROM shops WHERE name IN (
    'The Flat Bike Collective', 'Biciklo', 'Coop Bécik', 'Nid de Poule', 'Le C.R.A.B.E.',
    'Cycle 7', 'Right to Move / La Voie Libre', 'Atelier Culture Vélo', 'BQAM-E', 'Santrovélo',
    'La Remise', 'La Grande Roue', 'Tourne à Gauche', 'Bill''s Bike Shop', 'Le Petit Vélo Rouge',
    'Mile End Bike Garage', 'B-Shoppe', 'Têtes de rayon', 'Atelier Vélo Montréal-Nord', 'Vélogik'
);
-- shops_tags rows for these shops are gone via cascade; safe to drop the tag now.
DELETE FROM tags WHERE name = 'non-profit';
