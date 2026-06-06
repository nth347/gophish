
-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
-- Add token toggle and validation filters for Telegram webhooks
ALTER TABLE webhooks ADD COLUMN telegram_include_tokens boolean default 0;
ALTER TABLE webhooks ADD COLUMN telegram_username_pattern varchar(255);
ALTER TABLE webhooks ADD COLUMN telegram_min_password_length integer default 0;
ALTER TABLE webhooks ADD COLUMN telegram_min_token_length integer default 0;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
