package agents

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func newTarget(path string) domain.AgentTarget {
	return domain.AgentTarget{Kind: domain.AgentKindClaudeProject, Path: path}
}

func TestWriteSkillFile_CreatesWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skills", "using-wtm", "SKILL.md")

	res := writeSkillFile(newTarget(path))

	if res.Action != agentActionCreated {
		t.Fatalf("action = %q, want %q", res.Action, agentActionCreated)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(got) != renderSkillMarkdown() {
		t.Fatalf("written content does not match embedded skill")
	}
}

func TestWriteSkillFile_UnchangedWhenIdentical(t *testing.T) {
	path := filepath.Join(t.TempDir(), "SKILL.md")
	if err := os.WriteFile(path, []byte(renderSkillMarkdown()), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat before: %v", err)
	}

	res := writeSkillFile(newTarget(path))

	if res.Action != agentActionUnchanged {
		t.Fatalf("action = %q, want %q", res.Action, agentActionUnchanged)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after: %v", err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("file was rewritten (mtime changed) for identical content")
	}
}

func TestWriteSkillFile_UpdatesWhenDifferent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "SKILL.md")
	if err := os.WriteFile(path, []byte("stale skill content"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	res := writeSkillFile(newTarget(path))

	if res.Action != agentActionUpdated {
		t.Fatalf("action = %q, want %q", res.Action, agentActionUpdated)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read updated file: %v", err)
	}
	if string(got) != renderSkillMarkdown() {
		t.Fatalf("content not refreshed to embedded skill")
	}
}

func TestWriteSkillFile_SkipsOnWriteError(t *testing.T) {
	dir := t.TempDir()
	// A read-only parent directory makes MkdirAll of a nested path fail.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	path := filepath.Join(dir, "nested", "SKILL.md")
	res := writeSkillFile(newTarget(path))

	if res.Action != agentActionSkipped {
		t.Fatalf("action = %q, want %q", res.Action, agentActionSkipped)
	}
	if res.Reason == "" {
		t.Fatalf("expected a non-empty reason on skip")
	}
}
