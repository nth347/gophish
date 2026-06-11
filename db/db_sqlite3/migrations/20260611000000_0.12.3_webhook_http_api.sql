-- +goose Up
-- Add HTTP API webhook type fields: configurable HTTP method and custom headers.
ALTER TABLE webhooks ADD COLUMN api_method varchar(10) default 'POST';
ALTER TABLE webhooks ADD COLUMN api_headers text default '';

-- +goose Down
