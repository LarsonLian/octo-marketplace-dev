-- +migrate Up
ALTER TABLE parse_tasks ADD COLUMN attempts INT NOT NULL DEFAULT 0 AFTER file_sha256;

-- +migrate Down
ALTER TABLE parse_tasks DROP COLUMN attempts;
