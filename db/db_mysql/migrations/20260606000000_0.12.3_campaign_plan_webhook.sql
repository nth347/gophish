-- +goose Up
ALTER TABLE campaigns ADD COLUMN webhook_id bigint default 0;

-- +goose Down
ALTER TABLE campaigns DROP COLUMN webhook_id;
