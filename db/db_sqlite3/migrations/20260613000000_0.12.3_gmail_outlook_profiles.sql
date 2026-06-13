
-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
-- Add fields to support Gmail App Password and Outlook OAuth2 sending profiles
ALTER TABLE smtp ADD COLUMN outlook_client_id varchar(255);
ALTER TABLE smtp ADD COLUMN outlook_token_cache text;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
