-- +goose Up
CREATE TYPE submission_status AS ENUM ('pending', 'approved', 'rejected');

CREATE TABLE IF NOT EXISTS submissions (
    submission_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind VARCHAR(10) NOT NULL CHECK (kind IN ('create', 'update')),
    shop_id BIGINT REFERENCES shops(shop_id) ON DELETE CASCADE,
    payload JSONB NOT NULL,
    status submission_status NOT NULL DEFAULT 'pending',
    submitted_by VARCHAR(255) NOT NULL,
    reviewed_by VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ,

    CONSTRAINT update_needs_shop CHECK (kind = 'create' OR shop_id IS NOT NULL)
);

CREATE INDEX IF NOT EXISTS idx_submissions_status ON submissions(status);

-- +goose Down
DROP TABLE IF EXISTS submissions;
DROP TYPE IF EXISTS submission_status;
