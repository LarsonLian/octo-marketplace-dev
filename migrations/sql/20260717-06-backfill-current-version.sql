-- +migrate Up
-- DEV-39: backfill current_version_id from existing skill_versions
-- For skills that already have version records, pick the latest one.
UPDATE skills s
  JOIN (
    SELECT skill_id, id AS version_id
    FROM skill_versions sv1
    WHERE created_at = (
      SELECT MAX(created_at) FROM skill_versions sv2 WHERE sv2.skill_id = sv1.skill_id
    )
  ) latest ON latest.skill_id = s.id
SET s.current_version_id = latest.version_id
WHERE s.current_version_id = '';

-- For skills without any version record, create one from existing columns.
-- We use a deterministic UUID derived from the skill id for idempotency.
INSERT INTO skill_versions (id, skill_id, version, changelog, storage, changed_by, created_at)
SELECT
  CONCAT(SUBSTR(s.id, 1, 28), 'backfill') AS id,
  s.id AS skill_id,
  s.version,
  '自动回填',
  JSON_OBJECT(
    'type', 's3',
    'zip_object_key', s.file_url,
    'skill_md_object_key', '',
    'zip_file_name', s.file_name,
    'zip_size', s.file_size,
    'zip_sha256', s.file_sha256
  ),
  '' AS changed_by,
  s.created_at
FROM skills s
LEFT JOIN skill_versions sv ON sv.skill_id = s.id
WHERE sv.id IS NULL AND s.file_url != '';

-- Now backfill current_version_id for newly inserted records
UPDATE skills s
  JOIN skill_versions sv ON sv.skill_id = s.id
SET s.current_version_id = sv.id
WHERE s.current_version_id = '';

-- +migrate Down
-- Backfill data is left in place; clear the pointers only.
UPDATE skills SET current_version_id = '' WHERE current_version_id != '';
