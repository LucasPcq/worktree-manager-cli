package wt

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// readSourceBranch reads the recorded parent of a worktree from its meta.json.
func readSourceBranch(t *testing.T, stateDir, branch string) string {
	t.Helper()
	metaPath := filepath.Join(rules.WorktreeMetaDir(stateDir, branch), domain.MetaFileName)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read meta for %s: %v", branch, err)
	}
	var meta domain.WorktreeMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("unmarshal meta for %s: %v", branch, err)
	}
	return meta.SourceBranch
}

func setupStack(t *testing.T) (dir, stateDir string) {
	t.Helper()
	dir = gittest.InitRepo(t)
	stateDir = filepath.Join(dir, ".git", "wtm")
	t.Setenv("WTM_PROJECT_DIR", dir)
	t.Setenv("WTM_STATE_DIR", stateDir)
	t.Setenv(domain.EnvGoFile, "")

	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	for _, step := range []struct{ branch, from string }{
		{"feat", "main"},
		{"dev-a", "feat"},
		{"dev-b", "dev-a"},
	} {
		if _, _, err := runWtCmd(t, domain.CmdCreate, step.branch, "--from", step.from, "--output", domain.OutputJSON); err != nil {
			t.Fatalf("create %s: %v", step.branch, err)
		}
	}
	return dir, stateDir
}

func TestReparentUpdatesMetadata(t *testing.T) {
	_, stateDir := setupStack(t)

	if got := readSourceBranch(t, stateDir, "dev-b"); got != "dev-a" {
		t.Fatalf("precondition: dev-b parent = %q, want dev-a", got)
	}

	stdout, _, err := runWtCmd(t, domain.CmdReparent, "dev-b", "--to", "feat", "--output", domain.OutputJSON)
	if err != nil {
		t.Fatalf("reparent: %v", err)
	}

	var result domain.ReparentResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal reparent result: %v\n%s", err, stdout)
	}
	if result.OldParent != "dev-a" || result.NewParent != "feat" {
		t.Errorf("unexpected result: %+v", result)
	}

	if got := readSourceBranch(t, stateDir, "dev-b"); got != "feat" {
		t.Errorf("dev-b parent = %q, want feat", got)
	}
}

func TestReparentAcceptsRemoteParent(t *testing.T) {
	dir, stateDir := setupStack(t)

	// Fake an origin remote-tracking branch (no real remote needed).
	cmd := exec.Command("git", "update-ref", "refs/remotes/origin/staging", "HEAD")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git update-ref: %s: %v", out, err)
	}

	if _, _, err := runWtCmd(t, domain.CmdReparent, "dev-b", "--to", "origin/staging", "--output", domain.OutputJSON); err != nil {
		t.Fatalf("reparent onto remote parent: %v", err)
	}

	if got := readSourceBranch(t, stateDir, "dev-b"); got != "origin/staging" {
		t.Errorf("dev-b parent = %q, want origin/staging", got)
	}
}

func TestReparentRejectsUnknownParent(t *testing.T) {
	_, _ = setupStack(t)

	if _, _, err := runWtCmd(t, domain.CmdReparent, "dev-b", "--to", "does-not-exist", "--output", domain.OutputJSON); err == nil {
		t.Fatalf("expected an error reparenting onto a missing branch")
	}
}

func TestReparentRequiresParentInJSONMode(t *testing.T) {
	_, _ = setupStack(t)

	if _, _, err := runWtCmd(t, domain.CmdReparent, "dev-b", "--output", domain.OutputJSON); err == nil {
		t.Fatalf("expected a usage error without --to in JSON mode")
	}
}

func TestCleanReparentsChildrenWithFlag(t *testing.T) {
	_, stateDir := setupStack(t)

	if _, _, err := runWtCmd(t, domain.CmdClean, "dev-a", "--force", "--reparent-children", "--output", domain.OutputJSON); err != nil {
		t.Fatalf("clean dev-a: %v", err)
	}

	if got := readSourceBranch(t, stateDir, "dev-b"); got != "feat" {
		t.Errorf("dev-b parent = %q, want feat (grandparent)", got)
	}
}

func TestCleanLeavesChildrenOrphanedWithoutFlag(t *testing.T) {
	_, stateDir := setupStack(t)

	if _, _, err := runWtCmd(t, domain.CmdClean, "dev-a", "--force", "--output", domain.OutputJSON); err != nil {
		t.Fatalf("clean dev-a: %v", err)
	}

	if got := readSourceBranch(t, stateDir, "dev-b"); got != "dev-a" {
		t.Errorf("dev-b parent = %q, want dev-a (unchanged, no opt-in)", got)
	}
}
