-- +goose Up
-- "Tourne à Gauche" is the bike workshop program name bum.bike listed;
-- rename to the venue's actual brand name, Lespacemaker (lespacemaker.com).
UPDATE shops SET name = 'Lespacemaker', updated_at = NOW() WHERE name = 'Tourne à Gauche';

-- +goose Down
UPDATE shops SET name = 'Tourne à Gauche', updated_at = NOW() WHERE name = 'Lespacemaker';
