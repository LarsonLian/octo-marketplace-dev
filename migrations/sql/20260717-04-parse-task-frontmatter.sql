-- +migrate Up
-- DEV-39: extended frontmatter fields parsed from SKILL.md
ALTER TABLE parse_tasks ADD COLUMN result_id VARCHAR(36) NOT NULL DEFAULT '' AFTER result_readme;
ALTER TABLE parse_tasks ADD COLUMN result_forked_from VARCHAR(36) NOT NULL DEFAULT '' AFTER result_id;
ALTER TABLE parse_tasks ADD COLUMN result_metadata JSON NULL AFTER result_forked_from;

-- +migrate Down
ALTER TABLE parse_tasks DROP COLUMN result_metadata;
ALTER TABLE parse_tasks DROP COLUMN result_forked_from;
ALTER TABLE parse_tasks DROP COLUMN result_id;
