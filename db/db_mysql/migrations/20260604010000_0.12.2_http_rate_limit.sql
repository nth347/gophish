
-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
-- Add rate limiting and batching fields for HTTP API sending profiles
ALTER TABLE smtp ADD COLUMN http_rate_per_second integer default 0;
ALTER TABLE smtp ADD COLUMN http_rate_per_minute integer default 0;
ALTER TABLE smtp ADD COLUMN http_rate_per_hour integer default 0;
ALTER TABLE smtp ADD COLUMN http_batch_size integer default 0;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
