-- +migrate Up
ALTER TABLE mcp_servers
  ADD COLUMN verification_status ENUM('verified','unverified','error') NOT NULL DEFAULT 'unverified' AFTER transport,
  ADD COLUMN verified_at DATETIME(6) NULL AFTER verification_status,
  ADD INDEX idx_mcp_verification (verification_status, verified_at, updated_at, id);

-- JSON_SEARCH is acceptable below ~10k live MCPs. Before that threshold is
-- exceeded, backfill normalized mcp_tags/mcp_tools tables with FULLTEXT indexes
-- and dual-write them; switch reads only after result-parity verification.

-- +migrate Down
ALTER TABLE mcp_servers DROP INDEX idx_mcp_verification,
  DROP COLUMN verified_at, DROP COLUMN verification_status;
