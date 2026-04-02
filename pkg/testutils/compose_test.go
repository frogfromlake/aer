package testutils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetImageFromCompose_KnownService(t *testing.T) {
	image, err := GetImageFromCompose("postgres")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if image == "" {
		t.Fatal("expected non-empty image string")
	}
}

func TestGetImageFromFile_UnknownService(t *testing.T) {
	tmp := writeTempCompose(t, `
services:
  myservice:
    image: myimage:1.0
`)
	_, err := getImageFromFile(tmp, "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown service, got nil")
	}
}

func TestGetImageFromFile_MalformedYAML(t *testing.T) {
	tmp := writeTempCompose(t, "services: [invalid: yaml: {")
	_, err := getImageFromFile(tmp, "anything")
	if err == nil {
		t.Fatal("expected error for malformed YAML, got nil")
	}
}

func TestGetImageFromFile_MissingFile(t *testing.T) {
	_, err := getImageFromFile("/nonexistent/path/compose.yaml", "postgres")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func writeTempCompose(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "compose.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write temp compose file: %v", err)
	}
	return path
}
