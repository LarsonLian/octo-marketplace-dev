package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalStorage_PutObject(t *testing.T) {
	tmpDir := t.TempDir()
	ls := NewLocal(tmpDir, "http://localhost:8092")

	key := "skills/put-test-id/v1.0.0/skill.zip"
	content := "put object content here"

	err := ls.PutObject(context.Background(), key, strings.NewReader(content), int64(len(content)), "application/zip")
	if err != nil {
		t.Fatalf("PutObject failed: %v", err)
	}

	// Verify file exists on disk
	full := filepath.Join(tmpDir, key)
	data, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("file not found after PutObject: %v", err)
	}
	if string(data) != content {
		t.Errorf("file content = %q, want %q", data, content)
	}

	// GetObject should return the same content
	rc, err := ls.GetObject(context.Background(), key)
	if err != nil {
		t.Fatalf("GetObject after PutObject failed: %v", err)
	}
	defer rc.Close()
	got, _ := io.ReadAll(rc)
	if string(got) != content {
		t.Errorf("GetObject = %q, want %q", got, content)
	}
}

func TestLocalStorage_PutObject_CreatesDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	ls := NewLocal(tmpDir, "http://localhost:8092")

	// Deep nested path
	key := "skills/deep/nested/v2.0.0/sub/dir/file.md"
	content := "# Markdown"

	err := ls.PutObject(context.Background(), key, strings.NewReader(content), int64(len(content)), "text/markdown")
	if err != nil {
		t.Fatalf("PutObject with deep path failed: %v", err)
	}

	full := filepath.Join(tmpDir, key)
	data, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("file not found: %v", err)
	}
	if string(data) != content {
		t.Errorf("content = %q, want %q", data, content)
	}
}

func TestLocalStorage_PutObject_Overwrite(t *testing.T) {
	tmpDir := t.TempDir()
	ls := NewLocal(tmpDir, "http://localhost:8092")

	key := "skills/overwrite-id/v1.0.0/skill.zip"

	// Write initial
	err := ls.PutObject(context.Background(), key, strings.NewReader("initial"), 7, "application/zip")
	if err != nil {
		t.Fatal(err)
	}

	// Overwrite
	err = ls.PutObject(context.Background(), key, strings.NewReader("overwritten"), 11, "application/zip")
	if err != nil {
		t.Fatal(err)
	}

	// Verify overwritten content
	rc, err := ls.GetObject(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	got, _ := io.ReadAll(rc)
	if string(got) != "overwritten" {
		t.Errorf("content = %q, want %q", got, "overwritten")
	}
}

func TestLocalStorage_PutObject_RejectsTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	ls := NewLocal(tmpDir, "http://localhost:8092")

	traversalKeys := []string{
		"../../../etc/passwd",
		"skills/../../etc/passwd",
		"/etc/passwd",
	}

	for _, key := range traversalKeys {
		t.Run(key, func(t *testing.T) {
			err := ls.PutObject(context.Background(), key, strings.NewReader("evil"), 4, "text/plain")
			if err == nil {
				t.Errorf("PutObject(%q) should be rejected", key)
			}
		})
	}
}

func TestLocalStorage_PutObject_SkillPaths(t *testing.T) {
	tmpDir := t.TempDir()
	ls := NewLocal(tmpDir, "http://localhost:8092")

	// Test the actual path patterns used by the skill service
	tests := []struct {
		name string
		key  string
		ct   string
	}{
		{"zip upload", "skills/abc-123/v1.0.0/skill.zip", "application/zip"},
		{"SKILL.md upload", "skills/abc-123/v1.0.0/SKILL.md", "text/markdown; charset=utf-8"},
		{"temp upload", "skill-uploads/upload-id/skill.zip", "application/zip"},
		{"icon upload", "icons/icon-id/logo.png", "image/png"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := "test content for " + tt.key
			err := ls.PutObject(context.Background(), tt.key, strings.NewReader(content), int64(len(content)), tt.ct)
			if err != nil {
				t.Fatalf("PutObject(%q) failed: %v", tt.key, err)
			}

			// Verify content
			rc, err := ls.GetObject(context.Background(), tt.key)
			if err != nil {
				t.Fatalf("GetObject(%q) failed: %v", tt.key, err)
			}
			defer rc.Close()
			got, _ := io.ReadAll(rc)
			if string(got) != content {
				t.Errorf("GetObject = %q, want %q", got, content)
			}
		})
	}
}

func TestLocalStorage_CopyObject_AfterPut(t *testing.T) {
	tmpDir := t.TempDir()
	ls := NewLocal(tmpDir, "http://localhost:8092")

	srcKey := "skill-uploads/temp-id/skill.zip"
	dstKey := "skills/final-id/v1.0.0/skill.zip"
	content := "zip data"

	// Put to temp location
	err := ls.PutObject(context.Background(), srcKey, strings.NewReader(content), int64(len(content)), "application/zip")
	if err != nil {
		t.Fatal(err)
	}

	// Copy to permanent location
	err = ls.CopyObject(context.Background(), srcKey, dstKey)
	if err != nil {
		t.Fatal(err)
	}

	// Verify destination exists
	rc, err := ls.GetObject(context.Background(), dstKey)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	got, _ := io.ReadAll(rc)
	if string(got) != content {
		t.Errorf("copied content = %q, want %q", got, content)
	}

	// Source still exists
	rc2, err := ls.GetObject(context.Background(), srcKey)
	if err != nil {
		t.Fatal(err)
	}
	rc2.Close()
}
