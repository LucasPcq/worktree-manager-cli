package run

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadImportSourceFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "layout.json")
	content := []byte(`{"job":[],"profile":[]}`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := readImportSource([]string{path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("got %q, want %q", got, content)
	}
}

func TestReadImportSourceMissingFile(t *testing.T) {
	_, err := readImportSource([]string{"/nonexistent/path/layout.json"})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
