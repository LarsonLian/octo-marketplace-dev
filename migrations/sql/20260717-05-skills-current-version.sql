-- +migrate Up
-- DEV-39: add source_skill_id and current_version_id to skills
ALTER TABLE skills ADD COLUMN source_skill_id VARCHAR(36) NOT NULL DEFAULT '' AFTER icon_url;
ALTER TABLE skills ADD COLUMN current_version_id VARCHAR(36) NOT NULL DEFAULT '' AFTER source_skill_id;

-- +migrate Down
ALTER TABLE skills DROP COLUMN current_version_id;
ALTER TABLE skills DROP COLUMN source_skill_id;
