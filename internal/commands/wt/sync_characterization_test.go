package wt

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// These tests characterize `wtm sync` as it behaves today, before the flow/
// migration: which worktrees each entry selects, what --all, --dry-run,
// --push/--no-push and --yes resolve to, and the order of the cascade. They
// must pass unchanged after the migration.

type syncSetup struct {
	// Stack describes the worktrees to create, in order: each entry is
	// "branch:parent". The parent must already exist.
	Stack []string
}

type syncRepo struct {
	dir      string
	stateDir string
	remote   string
	paths    map[string]string
}

func setupSync(t *testing.T, setup syncSetup) syncRepo {
	t.Helper()
	repo := syncRepo{paths: map[string]string{}}
	repo.dir = gittest.InitRepo(t)
	repo.stateDir = filepath.Join(repo.dir, ".git", "wtm")
	t.Setenv("WTM_PROJECT_DIR", repo.dir)
	t.Setenv("WTM_STATE_DIR", repo.stateDir)
	t.Setenv(domain.EnvGoFile, "")

	if err := writeSyncConfig(repo.stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}
	repo.remote = gittest.AddOrigin(t, repo.dir)

	for _, entry := range setup.Stack {
		branch, parent, ok := strings.Cut(entry, ":")
		if !ok {
			t.Fatalf("stack entry %q must be \"branch:parent\"", entry)
		}
		repo.create(t, branch, parent)
	}
	return repo
}

func writeSyncConfig(stateDir string) error {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	content := `[worktrees]
base_path = "../.trees"
base_branch = "main"

[env]
strategy = "example"

[hooks]
on_create = []
on_clean = []
`
	return os.WriteFile(filepath.Join(stateDir, domain.ConfigFileName), []byte(content), 0o644)
}

func (r *syncRepo) create(t *testing.T, branch, from string) {
	t.Helper()
	stdout, _, err := runWtCmd(t, domain.CmdCreate, branch,
		"--from", from, "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("create %s: %v", branch, err)
	}
	var created struct {
		Path string `json:"path"`
	}
	if jsonErr := json.Unmarshal([]byte(stdout), &created); jsonErr != nil || created.Path == "" {
		t.Fatalf("create %s: cannot read the worktree path from %q", branch, stdout)
	}
	r.paths[branch] = created.Path
	gittest.PushBranch(t, r.dir, branch)
}

// commitOn adds a commit to a branch, in its worktree (or the main repo for the
// base), and publishes it when push is requested.
func (r syncRepo) commitOn(t *testing.T, branch, file string, push bool) {
	t.Helper()
	dir := r.paths[branch]
	if dir == "" {
		dir = r.dir
	}
	if err := os.WriteFile(filepath.Join(dir, file), []byte(file+"\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", file, err)
	}
	gittest.Git(t, dir, "add", file)
	gittest.Git(t, dir, "commit", "-m", "add "+file)
	if push {
		gittest.PushBranch(t, dir, branch)
	}
}

func (r syncRepo) tipOf(t *testing.T, branch string) string {
	t.Helper()
	dir := r.paths[branch]
	if dir == "" {
		dir = r.dir
	}
	out, err := exec.Command("git", "-C", dir, "rev-parse", branch).Output()
	if err != nil {
		t.Fatalf("rev-parse %s: %v", branch, err)
	}
	return strings.TrimSpace(string(out))
}

// syncJSON runs sync in machine output and returns the decoded result.
func syncJSON(t *testing.T, args ...string) domain.SyncResult {
	t.Helper()
	full := append([]string{domain.CmdSync}, args...)
	full = append(full, "--output", domain.OutputJSON, "--"+domain.FlagYes)
	stdout, _, err := runWtCmd(t, full...)
	if err != nil {
		t.Fatalf("sync %v: %v", args, err)
	}
	var result domain.SyncResult
	if jsonErr := json.Unmarshal([]byte(stdout), &result); jsonErr != nil {
		t.Fatalf("sync %v: cannot decode %q: %v", args, stdout, jsonErr)
	}
	return result
}

func TestSyncArgSelectsOnlyThatWorktree(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: []string{"feat-a:main", "feat-b:feat-a"}})
	repo.commitOn(t, "main", "base.txt", true)

	result := syncJSON(t, "feat-a")

	if len(result.Steps) != 1 || result.Steps[0].Branch != "feat-a" {
		t.Fatalf("sync feat-a must rebase feat-a alone, got %+v", result.Steps)
	}
}

func TestSyncAllSyncsEveryWorktree(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: []string{"feat-a:main", "feat-b:feat-a"}})
	repo.commitOn(t, "main", "base.txt", true)

	result := syncJSON(t, "--"+domain.FlagAll)

	if len(result.Steps) != 2 {
		t.Fatalf("--all must rebase every worktree, got %+v", result.Steps)
	}
}

func TestSyncAllRefusesBranchArguments(t *testing.T) {
	setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})

	_, _, err := runWtCmd(t, domain.CmdSync, "feat-a", "--"+domain.FlagAll, "--"+domain.FlagYes)
	if err == nil {
		t.Fatal("--all combined with branch arguments must be refused")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagAll) {
		t.Fatalf("the refusal must name --all, got: %v", err)
	}
}

func TestSyncPushAndNoPushAreMutuallyExclusive(t *testing.T) {
	setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})

	_, _, err := runWtCmd(t, domain.CmdSync, "--"+domain.FlagAll,
		"--"+domain.FlagPush, "--"+domain.FlagNoPush, "--"+domain.FlagYes)
	if err == nil {
		t.Fatal("--push and --no-push must be refused together")
	}
}

func TestSyncFFParentsAndNoFFParentsAreMutuallyExclusive(t *testing.T) {
	setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})

	_, _, err := runWtCmd(t, domain.CmdSync, "--"+domain.FlagAll,
		"--"+domain.FlagFFParents, "--"+domain.FlagNoFFParents, "--"+domain.FlagYes)
	if err == nil {
		t.Fatal("--ff-parents and --no-ff-parents must be refused together")
	}
}

func TestSyncYesWithoutTargetNamesAll(t *testing.T) {
	setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})

	_, _, err := runWtCmd(t, domain.CmdSync, "--"+domain.FlagYes)
	if err == nil {
		t.Fatal("--yes with no branch and no --all must be refused")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagAll) {
		t.Fatalf("the refusal must name --all, got: %v", err)
	}
}

func TestSyncJSONRequiresYesOrDryRun(t *testing.T) {
	setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})

	_, _, err := runWtCmd(t, domain.CmdSync, "--"+domain.FlagAll, "--output", domain.OutputJSON)
	if err == nil {
		t.Fatal("--output json without --yes or --dry-run must be refused")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagYes) {
		t.Fatalf("the refusal must name --yes, got: %v", err)
	}
}

func TestSyncDryRunRebasesNothing(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})
	repo.commitOn(t, "main", "base.txt", true)
	before := repo.tipOf(t, "feat-a")

	_, _, err := runWtCmd(t, domain.CmdSync, "--"+domain.FlagAll,
		"--"+domain.FlagDryRun, "--output", domain.OutputJSON)
	if err != nil {
		t.Fatalf("sync --dry-run: %v", err)
	}
	if after := repo.tipOf(t, "feat-a"); after != before {
		t.Fatalf("--dry-run moved feat-a: %s → %s", before, after)
	}
}

func TestSyncYesDoesNotPushByDefault(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})
	repo.commitOn(t, "main", "base.txt", true)

	result := syncJSON(t, "--"+domain.FlagAll)

	for _, step := range result.Steps {
		if step.Pushed {
			t.Fatalf("--yes must not push; %s was pushed", step.Branch)
		}
	}
}

func TestSyncPushForcePushesRebasedBranches(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})
	repo.commitOn(t, "main", "base.txt", true)

	result := syncJSON(t, "--"+domain.FlagAll, "--"+domain.FlagPush)

	pushed := false
	for _, step := range result.Steps {
		if step.Pushed {
			pushed = true
		}
	}
	if !pushed {
		t.Fatalf("--push must push the rebased branches, got %+v", result.Steps)
	}
}

func TestSyncNoPushNeverPushes(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})
	repo.commitOn(t, "main", "base.txt", true)

	result := syncJSON(t, "--"+domain.FlagAll, "--"+domain.FlagNoPush)

	for _, step := range result.Steps {
		if step.Pushed {
			t.Fatalf("--no-push must never push; %s was pushed", step.Branch)
		}
	}
}
