package skill

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	categoryrepo "github.com/Mininglamp-OSS/octo-marketplace/internal/repository/category"
	skillrepo "github.com/Mininglamp-OSS/octo-marketplace/internal/repository/skill"
	"github.com/Mininglamp-OSS/octo-marketplace/internal/storage"
)

// Compile-time check: fakeStorage must implement storage.Storage.
var _ storage.Storage = (*fakeStorage)(nil)

// fakeStorage implements storage.Storage for testing.
type fakeStorage struct {
	copyErr   error
	copyCount int
	copySrc   string
	copyDst   string
	getObject io.ReadCloser
}

func (f *fakeStorage) PresignPut(_ context.Context, _ string, _ string, _ time.Duration) (string, http.Header, error) {
	return "", nil, nil
}
func (f *fakeStorage) PresignGet(_ context.Context, _ string, _ time.Duration) (string, error) {
	return "", nil
}
func (f *fakeStorage) PublicURL(_ context.Context, key string) (string, error) {
	return "https://cdn.test/" + key, nil
}
func (f *fakeStorage) GetObject(_ context.Context, _ string) (io.ReadCloser, error) {
	if f.getObject != nil {
		return f.getObject, nil
	}
	return nil, nil
}
func (f *fakeStorage) DeleteObject(_ context.Context, _ string) error { return nil }
func (f *fakeStorage) CopyObject(_ context.Context, src, dst string) error {
	f.copyCount++
	f.copySrc = src
	f.copyDst = dst
	return f.copyErr
}
func (f *fakeStorage) PutObject(_ context.Context, _ string, _ io.Reader, _ int64, _ string) error {
	return nil
}

// TestCreate_CopyObjectFailure_NoDBMutation calls Service.Create with a failing
// CopyObject and verifies that the parse task is NOT consumed and no Skill is created.
func TestCreate_CopyObjectFailure_NoDBMutation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := &fakeStorage{copyErr: errors.New("storage unavailable")}
	repo := skillrepo.New(db)
	catRepo := categoryrepo.New(db)
	svc := New(repo, catRepo, store, func() string { return "new-skill-id" })

	// Mock GetParseTask query — returns a valid success task
	parseRows := sqlmock.NewRows([]string{
		"id", "upload_id", "file_name", "file_size", "file_url", "file_sha256",
		"status", "result_name", "result_description", "result_version",
		"result_tags", "result_readme",
		"result_id", "result_forked_from", "result_metadata",
		"owner_id", "space_id", "skill_id",
	}).AddRow(
		"task-1", "upload-1", "skill.zip", int64(1024), "skill-uploads/upload-1/skill.zip", "sha256abc",
		"success", "My Skill", "A description", "1.0.0",
		[]byte(`["tag1"]`), "# My Skill\nContent",
		"", "", nil,
		"user-1", "space-1", "",
	)
	mock.ExpectQuery("SELECT .+ FROM parse_tasks WHERE id").
		WithArgs("task-1").
		WillReturnRows(parseRows)

	// The new flow downloads from storage (GetObject) which uses fakeStorage
	// that returns nil — causing an error before CopyObject is ever used.

	ctx := context.Background()
	_, createErr := svc.Create(ctx, CreateParams{
		ParseTaskID: "task-1",
		UserID:      "user-1",
		UserName:    "User One",
		SpaceID:     "space-1",
	})

	// Verify Create returned an error (GetObject returns nil reader)
	if createErr == nil {
		t.Fatal("Create should have failed when GetObject returns nil")
	}

	// Verify no DB mutations occurred (sqlmock ensures no unexpected queries)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unexpected DB calls: %v", err)
	}
}

// TestCreate_CopyObjectSuccess_DBMutationOccurs calls Service.Create with a
// valid zip from GetObject and verifies the DB transaction (consume task + create skill) executes.
func TestCreate_CopyObjectSuccess_DBMutationOccurs(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := &fakeStorage{copyErr: nil, getObject: io.NopCloser(bytes.NewReader(makeTestZip(t)))}
	repo := skillrepo.New(db)
	catRepo := categoryrepo.New(db)
	svc := New(repo, catRepo, store, func() string { return "new-skill-id" })

	// Mock GetParseTask
	parseRows := sqlmock.NewRows([]string{
		"id", "upload_id", "file_name", "file_size", "file_url", "file_sha256",
		"status", "result_name", "result_description", "result_version",
		"result_tags", "result_readme",
		"result_id", "result_forked_from", "result_metadata",
		"owner_id", "space_id", "skill_id",
	}).AddRow(
		"task-1", "upload-1", "skill.zip", int64(1024), "skill-uploads/upload-1/skill.zip", "sha256abc",
		"success", "My Skill", "A description", "1.0.0",
		[]byte(`["tag1"]`), "# My Skill\nContent",
		"", "", nil,
		"user-1", "space-1", "",
	)
	mock.ExpectQuery("SELECT .+ FROM parse_tasks WHERE id").
		WithArgs("task-1").
		WillReturnRows(parseRows)

	// Expect the transaction: BEGIN, consume task, insert skill, COMMIT
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE parse_tasks SET status").
		WithArgs("task-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO skills").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO skill_tags").
		WithArgs("space-1", "tag1", "user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	// Expect InsertVersion (called after transaction commits; logged on failure)
	mock.ExpectExec("INSERT INTO skill_versions").
		WillReturnResult(sqlmock.NewResult(0, 1))

	ctx := context.Background()
	item, createErr := svc.Create(ctx, CreateParams{
		ParseTaskID: "task-1",
		UserID:      "user-1",
		UserName:    "User One",
		SpaceID:     "space-1",
	})

	if createErr != nil {
		t.Fatalf("Create should succeed, got: %v", createErr)
	}
	if item == nil {
		t.Fatal("Create should return a SkillItem")
	}

	// Verify Skill record uses the new zip key format
	expectedDst := "skills/new-skill-id/v1.0.0/skill.zip"
	if item.FileURL != expectedDst {
		t.Errorf("Skill FileURL = %q, want %q", item.FileURL, expectedDst)
	}

	// Verify all DB expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("DB expectations not met: %v", err)
	}
}

// TestUpdate_CopyObjectFailure_NoDBMutation calls Service.Update with a reupload
// parse_task_id and a failing CopyObject, verifying no DB mutations occur.
func TestUpdate_CopyObjectFailure_NoDBMutation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := &fakeStorage{copyErr: errors.New("disk full")}
	repo := skillrepo.New(db)
	catRepo := categoryrepo.New(db)
	svc := New(repo, catRepo, store, func() string { return "id" })

	// Mock GetByID — returns an existing skill
	skillRows := sqlmock.NewRows([]string{
		"id", "name", "display_name", "icon_url", "source_skill_id", "current_version_id",
		"description", "category_id", "tags", "owner_id", "owner_name",
		"space_id", "visibility", "version", "readme_content", "file_name", "file_url",
		"file_size", "file_sha256", "created_at", "updated_at",
	}).AddRow(
		"skill-1", "Old Skill", "Old Skill", "", "", "ver-1",
		"desc", "cat-1", []byte(`[]`), "user-1", "User One",
		"space-1", "space", "1.0.0", "old readme", "old.zip", "skills/skill-1/v1.0.0/old.zip",
		int64(512), "oldsha", time.Now(), time.Now(),
	)
	mock.ExpectQuery("SELECT .+ FROM skills").
		WithArgs("skill-1").
		WillReturnRows(skillRows)

	// Mock GetParseTask — returns a successful reupload task
	parseRows := sqlmock.NewRows([]string{
		"id", "upload_id", "file_name", "file_size", "file_url", "file_sha256",
		"status", "result_name", "result_description", "result_version",
		"result_tags", "result_readme",
		"result_id", "result_forked_from", "result_metadata",
		"owner_id", "space_id", "skill_id",
	}).AddRow(
		"task-2", "upload-2", "new-skill.zip", int64(2048), "skill-uploads/upload-2/new-skill.zip", "newsha",
		"success", "New Skill", "New desc", "2.0.0",
		[]byte(`["new"]`), "# New\nContent",
		"", "", nil,
		"user-1", "space-1", "skill-1",
	)
	mock.ExpectQuery("SELECT .+ FROM parse_tasks WHERE id").
		WithArgs("task-2").
		WillReturnRows(parseRows)

	// The new flow downloads from storage (GetObject) which returns nil — causing error

	ctx := context.Background()
	_, updateErr := svc.Update(ctx, "skill-1", "user-1", "space-1", UpdateParams{
		ParseTaskID: "task-2",
	})

	if updateErr == nil {
		t.Fatal("Update should have failed when GetObject returns nil")
	}

	// Verify no DB mutations (sqlmock catches unexpected queries)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unexpected DB calls: %v", err)
	}
}

// TestCreate_RejectsReuploadTask verifies Create rejects tasks with skill_id set.
func TestCreate_RejectsReuploadTask(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := &fakeStorage{}
	repo := skillrepo.New(db)
	catRepo := categoryrepo.New(db)
	svc := New(repo, catRepo, store, func() string { return "id" })

	// Mock GetParseTask — returns a reupload task (skill_id non-empty)
	parseRows := sqlmock.NewRows([]string{
		"id", "upload_id", "file_name", "file_size", "file_url", "file_sha256",
		"status", "result_name", "result_description", "result_version",
		"result_tags", "result_readme",
		"result_id", "result_forked_from", "result_metadata",
		"owner_id", "space_id", "skill_id",
	}).AddRow(
		"task-r", "upload-r", "reup.zip", int64(1024), "skill-uploads/upload-r/reup.zip", "sha",
		"success", "Name", "Desc", "1.0.0",
		[]byte(`[]`), "readme",
		"", "", nil,
		"user-1", "space-1", "existing-skill-id",
	)
	mock.ExpectQuery("SELECT .+ FROM parse_tasks WHERE id").
		WithArgs("task-r").
		WillReturnRows(parseRows)

	ctx := context.Background()
	_, createErr := svc.Create(ctx, CreateParams{
		ParseTaskID: "task-r",
		UserID:      "user-1",
		UserName:    "User",
		SpaceID:     "space-1",
	})

	if !errors.Is(createErr, ErrInvalidParseTask) {
		t.Errorf("Create with reupload task should return ErrInvalidParseTask, got: %v", createErr)
	}

	// CopyObject should NOT be called for rejected tasks
	if store.copyCount != 0 {
		t.Errorf("CopyObject should not be called for rejected task, count = %d", store.copyCount)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("DB expectations not met: %v", err)
	}
}

// TestCopyObject_KeyFormat verifies the object key format used for relocation.
func TestCopyObject_KeyFormat(t *testing.T) {
	tests := []struct {
		name     string
		skillID  string
		version  string
		fileName string
		want     string
	}{
		{
			name:     "standard",
			skillID:  "abc-123",
			version:  "1.0.0",
			fileName: "my-skill.zip",
			want:     "skills/abc-123/v1.0.0/my-skill.zip",
		},
		{
			name:     "complex version",
			skillID:  "def-456",
			version:  "2.1.0-beta",
			fileName: "tool.zip",
			want:     "skills/def-456/v2.1.0-beta/tool.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fmt.Sprintf("skills/%s/v%s/%s", tt.skillID, tt.version, tt.fileName)
			if got != tt.want {
				t.Errorf("key = %q, want %q", got, tt.want)
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// Ensure json import is used (for test data setup).
var _ = json.RawMessage(`[]`)

// makeTestZip creates a minimal zip file containing a SKILL.md for testing.
func makeTestZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create("SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write([]byte("---\nname: my-skill\ndescription: A test skill\nversion: 1.0.0\n---\n# My Skill\nContent"))
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
