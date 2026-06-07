-- +goose Up
CREATE TABLE IF NOT EXISTS `campaign_groups` (campaign_id bigint, group_id bigint);

-- +goose Down
DROP TABLE IF EXISTS `campaign_groups`;
