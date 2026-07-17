package skill

import "encoding/json"

// VersionStorage is the structured representation of the skill_versions.storage
// JSON column. Use this instead of hand-building JSON strings.
type VersionStorage struct {
	Type             string `json:"type"`
	ZipObjectKey     string `json:"zip_object_key"`
	SkillMdObjectKey string `json:"skill_md_object_key"`
	ZipFileName      string `json:"zip_file_name"`
	ZipSize          int64  `json:"zip_size"`
	ZipSHA256        string `json:"zip_sha256"`
}

// MarshalJSON returns the JSON encoding.
func (v VersionStorage) MarshalJSON() ([]byte, error) {
	type alias VersionStorage
	return json.Marshal(alias(v))
}

// ParseVersionStorage decodes a storage JSON string into VersionStorage.
// Falls back gracefully for legacy format {"type":"s3","object_key":"..."}.
func ParseVersionStorage(raw string) VersionStorage {
	if raw == "" {
		return VersionStorage{}
	}
	var vs VersionStorage
	if err := json.Unmarshal([]byte(raw), &vs); err != nil {
		return VersionStorage{}
	}
	// Fallback for legacy storage format
	if vs.ZipObjectKey == "" {
		var legacy struct {
			Type      string `json:"type"`
			ObjectKey string `json:"object_key"`
		}
		if err := json.Unmarshal([]byte(raw), &legacy); err == nil && legacy.ObjectKey != "" {
			vs.Type = legacy.Type
			vs.ZipObjectKey = legacy.ObjectKey
		}
	}
	return vs
}

// JSON serializes the VersionStorage to a JSON string for DB storage.
func (v VersionStorage) JSON() string {
	b, _ := json.Marshal(v)
	return string(b)
}
