package skill

import (
	"context"
	"database/sql"
)

// GetByID returns a single skill by ID. Returns nil if not found.
func (r *Repo) GetByID(ctx context.Context, id string) (*SkillRow, error) {
	query := `
		SELECT s.id, s.name, s.display_name, s.icon_url, s.source_skill_id, s.current_version_id,
			s.description, s.category_id, s.tags,
			s.owner_id, s.owner_name, s.space_id, s.visibility, s.version,
			s.readme_content, s.file_name, s.file_url, s.file_size, s.file_sha256,
			s.created_at, s.updated_at,
			COALESCE(v.version, s.version) AS resolved_version,
			COALESCE(v.storage, '') AS version_storage
		FROM skills s
		LEFT JOIN skill_versions v ON v.id = s.current_version_id
		WHERE s.id = ?
	`
	var s SkillRow
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&s.ID, &s.Name, &s.DisplayName, &s.IconURL, &s.SourceSkillID, &s.CurrentVersionID,
		&s.Description, &s.CategoryID, &s.Tags,
		&s.OwnerID, &s.OwnerName, &s.SpaceID, &s.Visibility, &s.Version,
		&s.ReadmeContent, &s.FileName, &s.FileURL, &s.FileSize, &s.FileSHA256,
		&s.CreatedAt, &s.UpdatedAt,
		&s.ResolvedVersion, &s.VersionStorage,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}
