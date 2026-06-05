
-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
-- Allow Telegram webhooks to include captured username/password for submitted data
ALTER TABLE webhooks ADD COLUMN telegram_include_username boolean default 0;
ALTER TABLE webhooks ADD COLUMN telegram_include_password boolean default 0;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
