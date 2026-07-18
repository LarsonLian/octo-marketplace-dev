package skill

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestBuildRewrittenSkillMD_PreservesOriginalFields(t *testing.T) {
	// Original SKILL.md has user/vendor metadata fields
	original := []byte("---\nname: old-name\ndescription: old desc\nversion: 0.9.0\nauthor: someone\nlicense: MIT\nmetadata:\n  openclaw:\n    category: tools\n  octo:\n    space_id: sp-123\ncustom_field: hello\n---\n# My Skill\n\nBody content here.\n")

	result := buildRewrittenSkillMD(original, RewriteParams{
		Name:       "new-name",
		Desc:       "new description",
		Version:    "1.0.0",
		Tags:       []string{"tag1", "tag2"},
		ID:         "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		ForkedFrom: "11111111-2222-3333-4444-555555555555",
	})

	text := string(result)

	// Should preserve body
	if !strings.Contains(text, "# My Skill") {
		t.Error("body content should be preserved")
	}
	if !strings.Contains(text, "Body content here.") {
		t.Error("body text should be preserved")
	}

	// Parse frontmatter from result
	fmYAML, _ := splitFrontmatterAndBody(result)
	var fm map[string]interface{}
	if err := yaml.Unmarshal([]byte(fmYAML), &fm); err != nil {
		t.Fatalf("failed to parse result frontmatter: %v", err)
	}

	// Machine fields should be overwritten
	if fm["name"] != "new-name" {
		t.Errorf("name = %v, want new-name", fm["name"])
	}
	if fm["description"] != "new description" {
		t.Errorf("description = %v, want 'new description'", fm["description"])
	}
	if fm["version"] != "1.0.0" {
		t.Errorf("version = %v, want 1.0.0", fm["version"])
	}
	if fm["id"] != "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" {
		t.Errorf("id = %v, want aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", fm["id"])
	}
	if fm["forked_from"] != "11111111-2222-3333-4444-555555555555" {
		t.Errorf("forked_from = %v, want 11111111-2222-3333-4444-555555555555", fm["forked_from"])
	}
	tags, ok := fm["tags"].([]interface{})
	if !ok || len(tags) != 2 {
		t.Errorf("tags = %v, want [tag1, tag2]", fm["tags"])
	}

	// Original user/vendor fields should be preserved
	if fm["author"] != "someone" {
		t.Errorf("author should be preserved, got %v", fm["author"])
	}
	if fm["license"] != "MIT" {
		t.Errorf("license should be preserved, got %v", fm["license"])
	}
	if fm["custom_field"] != "hello" {
		t.Errorf("custom_field should be preserved, got %v", fm["custom_field"])
	}

	// Nested metadata should be preserved (including all sub-keys)
	metadata, ok := fm["metadata"].(map[string]interface{})
	if !ok {
		t.Fatalf("metadata should be preserved as map, got %T: %v", fm["metadata"], fm["metadata"])
	}
	if _, ok := metadata["openclaw"]; !ok {
		t.Error("metadata.openclaw should be preserved")
	}
	if _, ok := metadata["octo"]; !ok {
		t.Error("metadata.octo should be preserved")
	}
}

func TestBuildRewrittenSkillMD_NoOriginalFrontmatter(t *testing.T) {
	// SKILL.md without frontmatter
	original := []byte("# Just a body\n\nNo frontmatter here.\n")

	result := buildRewrittenSkillMD(original, RewriteParams{
		Name:    "my-skill",
		Desc:    "A skill",
		Version: "1.0.0",
		ID:      "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
	})

	fmYAML, body := splitFrontmatterAndBody(result)
	var fm map[string]interface{}
	if err := yaml.Unmarshal([]byte(fmYAML), &fm); err != nil {
		t.Fatalf("failed to parse frontmatter: %v", err)
	}
	if fm["name"] != "my-skill" {
		t.Errorf("name = %v", fm["name"])
	}
	if fm["id"] != "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" {
		t.Errorf("id = %v", fm["id"])
	}
	if !strings.Contains(body, "# Just a body") {
		t.Error("body should be preserved")
	}
}

func TestBuildRewrittenSkillMD_EmptyForkedFromRemoved(t *testing.T) {
	original := []byte("---\nname: x\nforked_from: old-uuid\n---\nbody\n")

	result := buildRewrittenSkillMD(original, RewriteParams{
		Name:       "x",
		Desc:       "d",
		Version:    "1.0.0",
		ForkedFrom: "", // empty => remove
	})

	fmYAML, _ := splitFrontmatterAndBody(result)
	var fm map[string]interface{}
	_ = yaml.Unmarshal([]byte(fmYAML), &fm)
	if _, ok := fm["forked_from"]; ok {
		t.Error("forked_from should be removed when empty")
	}
}

func TestRewriteZipPackage_PreservesNonSkillMDFiles(t *testing.T) {
	// Create a zip with SKILL.md and another file
	var zipBuf bytes.Buffer
	w := zip.NewWriter(&zipBuf)
	skillMD, _ := w.Create("SKILL.md")
	skillMD.Write([]byte("---\nname: old\ndescription: old desc\nversion: 0.1.0\ncustom: keep-me\n---\n# Body\n"))
	otherFile, _ := w.Create("README.md")
	otherFile.Write([]byte("# Other content\nDo not touch.\n"))
	w.Close()

	zipBytes := zipBuf.Bytes()
	result, err := RewriteZipPackage(
		bytes.NewReader(zipBytes), int64(len(zipBytes)),
		RewriteParams{
			Name:    "new-name",
			Desc:    "new desc",
			Version: "2.0.0",
			ID:      "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		},
	)
	if err != nil {
		t.Fatalf("RewriteZipPackage failed: %v", err)
	}

	// Verify the rewritten zip
	reader, err := zip.NewReader(bytes.NewReader(result.ZipBytes), result.ZipSize)
	if err != nil {
		t.Fatalf("failed to read result zip: %v", err)
	}

	foundSkillMD := false
	foundOther := false
	for _, f := range reader.File {
		rc, _ := f.Open()
		data, _ := io.ReadAll(rc)
		rc.Close()
		content := string(data)

		if f.Name == "SKILL.md" {
			foundSkillMD = true
			// Verify machine fields rewritten
			if !strings.Contains(content, "new-name") {
				t.Errorf("SKILL.md should have rewritten name, got:\n%s", content)
			}
			// Verify custom field preserved
			if !strings.Contains(content, "custom: keep-me") {
				t.Errorf("SKILL.md should preserve custom field, got:\n%s", content)
			}
			// Verify body preserved
			if !strings.Contains(content, "# Body") {
				t.Errorf("SKILL.md should preserve body, got:\n%s", content)
			}
		}
		if f.Name == "README.md" {
			foundOther = true
			if !strings.Contains(content, "Do not touch.") {
				t.Errorf("README.md should be unchanged, got:\n%s", content)
			}
		}
	}

	if !foundSkillMD {
		t.Error("SKILL.md not found in rewritten zip")
	}
	if !foundOther {
		t.Error("README.md not found in rewritten zip")
	}

	// Verify SkillMD output
	fmYAML, _ := splitFrontmatterAndBody(result.SkillMD)
	var fm map[string]interface{}
	_ = yaml.Unmarshal([]byte(fmYAML), &fm)
	if fm["custom"] != "keep-me" {
		t.Errorf("SkillMD output should preserve custom field, got %v", fm["custom"])
	}
	if fm["id"] != "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" {
		t.Errorf("SkillMD output should have injected id, got %v", fm["id"])
	}
}
