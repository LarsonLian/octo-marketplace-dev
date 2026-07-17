package skill

import (
	"context"
	"database/sql"
	"encoding/json"
)

// GetByID returns a single skill by ID with current version data. Returns nil if not found.
// When current_version_id is set, file fields are resolved from skill_versions.storage JSON.
func (r *Repo) GetByID(ctx context.Context, id string) (*SkillRow, error) {
	query := `
		SELECT s.id, s.name, s.display_name, s.icon_url, s.source_skill_id, s.current_version_id,
			s.description, s.category_id, s.tags,
			s.owner_id, s.owner_name, s.space_id, s.visibility,
			COALESCE(v.version, s.version),
			s.readme_content, s.file_name, s.file_url, s.file_size, s.file_sha256,
			s.created_at, s.updated_at,
			v.storage
		FROM skills s
		LEFT JOIN skill_versions v ON v.id = s.current_version_id
		WHERE s.id = ?
	`
	var s SkillRow
	var storageJSON sql.NullString
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&s.ID, &s.Name, &s.DisplayName, &s.IconURL, &s.SourceSkillID, &s.CurrentVersionID,
		&s.Description, &s.CategoryID, &s.Tags,
		&s.OwnerID, &s.OwnerName, &s.SpaceID, &s.Visibility, &s.Version,
		&s.ReadmeContent, &s.FileName, &s.FileURL, &s.FileSize, &s.FileSHA256,
		&s.CreatedAt, &s.UpdatedAt,
		&storageJSON,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Override file fields from version storage when available
	if storageJSON.Valid && storageJSON.String != "" {
		s.StorageJSON = storageJSON.String
		applyVersionStorage(&s, storageJSON.String)
	}

	return &s, nil
}

// applyVersionStorage overrides SkillRow file fields from the version storage
// JSON, supporting both the new schema and legacy {"type":"s3","object_key":"..."}.
func applyVersionStorage(s *SkillRow, raw string) {
	vs := parseStorageJSON(raw)
	if vs.ZipObjectKey != "" {
		s.FileURL = vs.ZipObjectKey
	}
	if vs.ZipFileName != "" {
		s.FileName = vs.ZipFileName
	}
	if vs.ZipSize > 0 {
		s.FileSize = vs.ZipSize
	}
	if vs.ZipSHA256 != "" {
		s.FileSHA256 = vs.ZipSHA256
	}
}

// parseStorageJSON decodes the storage JSON, supporting legacy format.
func parseStorageJSON(raw string) versionStorageData {
	if raw == "" {
		return versionStorageData{}
	}
	var vs versionStorageData
	if err := json.Unmarshal([]byte(raw), &vs); err != nil {
		return versionStorageData{}
	}
	// Fallback for legacy format
	if vs.ZipObjectKey == "" {
		var legacy struct {
			ObjectKey string `json:"object_key"`
		}
		if err := json.Unmarshal([]byte(raw), &legacy); err == nil && legacy.ObjectKey != "" {
			vs.ZipObjectKey = legacy.ObjectKey
		}
	}
	return vs
}

// versionStorageData mirrors the JSON structure in skill_versions.storage.
type versionStorageData struct {
	Type             string `json:"type"`
	ZipObjectKey     string `json:"zip_object_key"`
	SkillMdObjectKey string `json:"skill_md_object_key"`
	ZipFileName      string `json:"zip_file_name"`
	ZipSize          int64  `json:"zip_size"`
	ZipSHA256        string `json:"zip_sha256"`
}
