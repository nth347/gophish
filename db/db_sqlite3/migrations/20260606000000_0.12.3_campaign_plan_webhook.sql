-- +goose Up
-- Add webhook_id to campaigns to allow per-campaign notification routing.
-- A value of 0 means "use global active webhooks" (backwards compatible).
ALTER TABLE campaigns ADD COLUMN webhook_id integer default 0;

-- +goose Down
