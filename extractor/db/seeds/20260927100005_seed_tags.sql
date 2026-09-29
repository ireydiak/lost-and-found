-- +goose Up
INSERT INTO tags (name) VALUES
    ('stolen'),
    ('abandoned'),
    ('other');

-- +goose Down
DELETE FROM tags;
