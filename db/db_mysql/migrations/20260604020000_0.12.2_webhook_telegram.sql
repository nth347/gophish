
-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
-- Add Telegram support and per-event filtering to webhooks
ALTER TABLE webhooks ADD COLUMN type varchar(255) default 'standard';
ALTER TABLE webhooks ADD COLUMN telegram_bot_token varchar(255);
ALTER TABLE webhooks ADD COLUMN telegram_chat_id varchar(255);
ALTER TABLE webhooks ADD COLUMN events varchar(255);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
