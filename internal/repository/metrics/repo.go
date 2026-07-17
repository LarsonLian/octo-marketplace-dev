package metrics

import (
	"context"
	"database/sql"
	"fmt"
)

// Repo handles persistence for resource_metrics.
type Repo struct {
	db *sql.DB
}

// New creates a new metrics Repo.
func New(db *sql.DB) *Repo {
	return &Repo{db: db}
}

// UpsertCounts atomically upserts view/download/install deltas for a resource.
// ON DUPLICATE KEY UPDATE accumulates the deltas into the existing counts.
func (r *Repo) UpsertCounts(ctx context.Context, resourceType, resourceID string, viewDelta, downloadDelta, installDelta int64) error {
	const query = `INSERT INTO resource_metrics (resource_type, resource_id, view_count, download_count, install_count)
VALUES (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  view_count = view_count + VALUES(view_count),
  download_count = download_count + VALUES(download_count),
  install_count = install_count + VALUES(install_count)`

	_, err := r.db.ExecContext(ctx, query, resourceType, resourceID, viewDelta, downloadDelta, installDelta)
	if err != nil {
		return fmt.Errorf("upsert resource_metrics %s/%s: %w", resourceType, resourceID, err)
	}
	return nil
}
