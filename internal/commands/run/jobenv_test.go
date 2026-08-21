package run

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
)

// startEnv is the environment the commands asked the daemon to start the named
// job with — what a job actually receives, read off the wire.
func (d *fakeDaemon) startEnv(t *testing.T, job string) map[string]string {
	t.Helper()
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, req := range d.requests {
		if req.Action == process.ActionStart && req.Job != nil && req.Job.Name == job {
			return req.Env
		}
	}
	t.Fatalf("the daemon was never asked to start %q", job)
	return nil
}

func addWorktree(t *testing.T, projectDir, branch string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), strings.ReplaceAll(branch, "/", "-"))
	cmd := exec.Command("git", "worktree", "add", "-q", "-b", branch, path, "HEAD")
	cmd.Dir = projectDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %s: %v", out, err)
	}
	return path
}

func enterWorktree(t *testing.T, path string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(path); err != nil {
		t.Fatalf("chdir %q: %v", path, err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}

// composeProject is the name wtm derives for a worktree of the test project:
// the repository's own directory qualifies it, so two clones on the same branch
// do not share a stack.
func composeProject(t *testing.T, worktreeSlug string) string {
	t.Helper()
	return rules.ComposeProjectName(rules.ComposeProjectNameParams{
		Project:  filepath.Base(os.Getenv("WTM_PROJECT_DIR")),
		Worktree: worktreeSlug,
	})
}

func assertEnv(t *testing.T, env map[string]string, want map[string]string) {
	t.Helper()
	for key, value := range want {
		if env[key] != value {
			t.Errorf("%s = %q, want %q", key, env[key], value)
		}
	}
}

func TestRunUpInjectsMainWorktreeEnv(t *testing.T) {
	daemon := setupUpProject(t, &fakeDaemon{})
	fakeTTY(t, false)
	enterWorktree(t, os.Getenv("WTM_PROJECT_DIR"))

	if _, _, err := runCmd(t, domain.CmdUp, "--"+domain.FlagDetach); err != nil {
		t.Fatalf("run up: %v", err)
	}

	// The main checkout keeps the project's default ports.
	assertEnv(t, daemon.startEnv(t, "docker"), map[string]string{
		domain.EnvBranch:             "main",
		domain.EnvWorktree:           "main",
		domain.EnvOrdinal:            "0",
		domain.EnvPortOffset:         "0",
		domain.EnvComposeProjectName: composeProject(t, "main"),
	})
}

func TestRunUpInjectsLinkedWorktreeEnv(t *testing.T) {
	daemon := setupUpProject(t, &fakeDaemon{})
	fakeTTY(t, false)
	enterWorktree(t, addWorktree(t, os.Getenv("WTM_PROJECT_DIR"), "feat/x"))

	if _, _, err := runCmd(t, domain.CmdUp, "--"+domain.FlagDetach); err != nil {
		t.Fatalf("run up: %v", err)
	}

	assertEnv(t, daemon.startEnv(t, "docker"), map[string]string{
		domain.EnvBranch:             "feat/x",
		domain.EnvWorktree:           "feat-x",
		domain.EnvOrdinal:            "1",
		domain.EnvPortOffset:         "10",
		domain.EnvComposeProjectName: composeProject(t, "feat-x"),
	})
}

// run start bypasses flow/runlogs entirely; it must carry the same env.
func TestRunStartInjectsWorktreeEnv(t *testing.T) {
	daemon := setupUpProject(t, &fakeDaemon{})
	fakeTTY(t, false)
	enterWorktree(t, addWorktree(t, os.Getenv("WTM_PROJECT_DIR"), "feat/y"))

	if _, _, err := runCmd(t, domain.CmdStart, "api", "--"+domain.FlagDetach); err != nil {
		t.Fatalf("run start: %v", err)
	}

	assertEnv(t, daemon.startEnv(t, "api"), map[string]string{
		domain.EnvBranch:     "feat/y",
		domain.EnvOrdinal:    "1",
		domain.EnvPortOffset: "10",
	})
}

func TestRunUpKeepsUserComposeProjectName(t *testing.T) {
	daemon := setupUpProject(t, &fakeDaemon{})
	fakeTTY(t, false)
	enterWorktree(t, addWorktree(t, os.Getenv("WTM_PROJECT_DIR"), "feat/z"))
	t.Setenv(domain.EnvComposeProjectName, "perso")

	if _, _, err := runCmd(t, domain.CmdUp, "--"+domain.FlagDetach); err != nil {
		t.Fatalf("run up: %v", err)
	}

	assertEnv(t, daemon.startEnv(t, "docker"), map[string]string{
		domain.EnvComposeProjectName: "perso",
		domain.EnvOrdinal:            "1",
	})
}
