package worktree

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestWriteMetadata(t *testing.T) {
	dir := t.TempDir()

	metadata := domain.WorktreeMetadata{
		SourceBranch: "main",
		CreatedAt:    "2026-04-01T18:00:00Z",
		EnvStrategy:  domain.EnvStrategyExample,
	}

	if err := writeMetadata(dir, metadata); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify meta.json
	metaPath := filepath.Join(dir, domain.ProjectDirName, domain.MetaFileName)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("meta.json not found: %v", err)
	}

	var parsed domain.WorktreeMetadata
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if parsed.SourceBranch != "main" {
		t.Errorf("expected source_branch=main, got %s", parsed.SourceBranch)
	}
	if parsed.EnvStrategy != domain.EnvStrategyExample {
		t.Errorf("expected env_strategy=example, got %s", parsed.EnvStrategy)
	}

	// Verify context.md exists
	contextPath := filepath.Join(dir, domain.ProjectDirName, domain.ContextFileName)
	if _, err := os.Stat(contextPath); os.IsNotExist(err) {
		t.Error("context.md not found")
	}
}
