
-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
-- Add fields to support HTTP API sending profiles
ALTER TABLE smtp ADD COLUMN http_method varchar(255);
ALTER TABLE smtp ADD COLUMN http_url varchar(255);
ALTER TABLE smtp ADD COLUMN http_headers text;
ALTER TABLE smtp ADD COLUMN http_content_type varchar(255);
ALTER TABLE smtp ADD COLUMN http_body text;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
