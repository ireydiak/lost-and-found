-- +goose Up
INSERT INTO tags (name, display_name) VALUES
    ('bike-shop', 'Bike Shop'),
    ('used-bikes', 'Used Bikes'),
    ('repair', 'Repair'),
    ('rental', 'Rental'),
    ('accessories', 'Accessories'),
    ('custom-builds', 'Custom Builds'),
    ('sporting-goods', 'Sporting Goods'),
    ('unknown', 'Unknown'),
    ('pawn-shop', 'Pawn Shop');

-- +goose Down
DELETE FROM tags;
