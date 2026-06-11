-- +goose Up
-- Add per-field validation options for HTTP API webhooks, mirroring the
-- Telegram webhook filters.
ALTER TABLE webhooks ADD COLUMN api_include_username boolean default false;
ALTER TABLE webhooks ADD COLUMN api_include_password boolean default false;
ALTER TABLE webhooks ADD COLUMN api_include_tokens boolean default false;
ALTER TABLE webhooks ADD COLUMN api_username_pattern varchar(255) default '';
ALTER TABLE webhooks ADD COLUMN api_min_password_length integer default 0;
ALTER TABLE webhooks ADD COLUMN api_min_token_length integer default 0;

-- +goose Down
