
-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
-- Add per-profile sending rate limit fields (applies to all interface types)
ALTER TABLE smtp ADD COLUMN max_send_attempts integer NOT NULL DEFAULT 0;
ALTER TABLE smtp ADD COLUMN send_delay integer NOT NULL DEFAULT 0;
ALTER TABLE smtp ADD COLUMN send_jitter_pct integer NOT NULL DEFAULT 0;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
