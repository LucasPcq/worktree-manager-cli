package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAgentTargets_DetectsProjectArtifacts(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := AgentTargets(project)
	byKind := map[AgentKind]AgentTarget{}
	for _, tg := range got {
		byKind[tg.Kind] = tg
	}

	if !byKind[AgentKindClaudeProject].Exists {
		t.Error("expected project .claude to be detected")
	}
	if byKind[AgentKindCursorProject].Exists {
		t.Error("cursor project should not be detected")
	}
	if byKind[AgentKindClaudeGlobal].Exists {
		t.Error("global .claude should not exist in tmp HOME")
	}
	if byKind[AgentKindClaudeProject].IsGlobal {
		t.Error("project target flagged as global")
	}
	if !byKind[AgentKindClaudeGlobal].IsGlobal {
		t.Error("global target not flagged as global")
	}
}

func TestAgentTargets_AllTargetsPointAtSkillMd(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	got := AgentTargets(t.TempDir())
	if len(got) != 4 {
		t.Fatalf("expected 4 targets (claude+cursor × project+global), got %d", len(got))
	}
	for _, tg := range got {
		if filepath.Base(tg.Path) != "SKILL.md" {
			t.Errorf("%s: expected SKILL.md artifact, got %s", tg.Kind, tg.Path)
		}
	}
}
