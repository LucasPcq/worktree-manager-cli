package branch

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
}

func revParse(t *testing.T, dir, ref string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse %s: %v", ref, err)
	}
	return strings.TrimSpace(string(out))
}

// repoWithRemote returns a work repo (on main) wired to a fresh bare origin, with
// main pushed so origin-tracking refs exist.
func repoWithRemote(t *testing.T) string {
	t.Helper()
	work := gittest.InitRepo(t)
	origin := t.TempDir()
	cmd := exec.Command("git", "init", "--bare", origin)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %s: %v", out, err)
	}
	git(t, work, "remote", "add", "origin", origin)
	git(t, work, "push", "-u", "origin", "main")
	return work
}

func TestFastForwardToOriginAdvancesBehindBranch(t *testing.T) {
	work := repoWithRemote(t)

	// "feat" at base, pushed; origin then advances feat by one commit while the
	// local feat stays put and unchecked out (HEAD on main).
	git(t, work, "branch", "feat")
	git(t, work, "push", "origin", "feat")
	git(t, work, "commit", "--allow-empty", "-m", "server-commit")
	git(t, work, "push", "origin", "main:feat")

	if err := FastForwardToOrigin(BranchParams{ProjectDir: work, Branch: "feat"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got, want := revParse(t, work, "feat"), revParse(t, work, "origin/feat"); got != want {
		t.Errorf("feat = %s, want origin/feat = %s", got, want)
	}
}

func TestFastForwardToOriginRefusesDiverged(t *testing.T) {
	work := repoWithRemote(t)

	git(t, work, "branch", "feat")
	git(t, work, "push", "origin", "feat")

	// local feat gains a commit...
	git(t, work, "checkout", "feat")
	git(t, work, "commit", "--allow-empty", "-m", "local-commit")
	local := revParse(t, work, "feat")
	git(t, work, "checkout", "main")

	// ...and origin's feat gains a different commit → diverged.
	git(t, work, "commit", "--allow-empty", "-m", "server-commit")
	git(t, work, "push", "origin", "main:feat")

	err := FastForwardToOrigin(BranchParams{ProjectDir: work, Branch: "feat"})
	if err == nil {
		t.Fatal("expected a diverged error, got nil")
	}
	if got := revParse(t, work, "feat"); got != local {
		t.Errorf("feat was modified (%s), want unchanged %s", got, local)
	}
}
