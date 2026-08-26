package wt

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	newpicker "github.com/LucasPcq/wtm/internal/tui/newwt"
)

// These tests characterize the command-layer deciders the create wizard is built
// on: which branch a fast-forward applies to, how divergence is classified into a
// prompt, and how the recap path is rendered. They are the git-facing half of the
// wizard path (internal/tui/newwt/create_flow_test.go covers the step half).

// promptFor classifies a branch pair the way the wizard does.
func promptFor(t *testing.T, projectDir, branchName, source string) newpicker.SourceUpdatePrompt {
	t.Helper()
	return sourceUpdatePrompt(sourceUpdatePromptParams{
		ProjectDir: projectDir,
		Target:     memoizedTarget(projectDir),
		Update:     newpicker.SourceUpdateParams{Branch: branchName, Source: source},
	})
}

// TestSourceUpdatePromptOffersFastForwardWhenBehind: a behind-only source is an
// actionable offer whose decline simply proceeds.
func TestSourceUpdatePromptOffersFastForwardWhenBehind(t *testing.T) {
	work := repoWithRemote(t)
	behindBranch(t, work, "feat/behind")

	got := promptFor(t, work, "", "feat/behind")
	if !got.Show {
		t.Fatalf("prompt = %+v, want a fast-forward offer for a behind branch", got)
	}
	if got.AbortOnDecline {
		t.Error("declining a fast-forward offer must not cancel the creation")
	}
	if got.Branch != "feat/behind" {
		t.Errorf("Branch = %q, want the source branch", got.Branch)
	}
}

// TestSourceUpdatePromptWarnsWhenDiverged: a diverged source cannot be
// fast-forwarded, so it is a hard confirmation carrying the warning.
func TestSourceUpdatePromptWarnsWhenDiverged(t *testing.T) {
	work := repoWithRemote(t)
	divergedBranch(t, work, "feat/diverged")

	got := promptFor(t, work, "", "feat/diverged")
	if !got.Show || !got.AbortOnDecline {
		t.Fatalf("prompt = %+v, want a hard confirmation for a diverged source", got)
	}
	if got.Params.Warning != domain.SourceDivergedWarning {
		t.Errorf("Warning = %q, want the diverged warning", got.Params.Warning)
	}
	if got.SkipReason == "" {
		t.Error("a diverged source needs a skip reason: the wizard shows it in the recap, not as a step")
	}
}

// TestSourceUpdatePromptSkipsWhenNothingToReconcile covers the three no-op cases:
// up to date, a remote start-point, and no source at all.
func TestSourceUpdatePromptSkipsWhenNothingToReconcile(t *testing.T) {
	work := repoWithRemote(t)

	tests := []struct {
		name   string
		source string
	}{
		{"up to date", "main"},
		{"remote start-point", "origin/main"},
		{"no source", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := promptFor(t, work, "", tt.source)
			if got.Show {
				t.Errorf("prompt = %+v, want no step for a %s source", got, tt.name)
			}
			if got.SkipReason == "" {
				t.Error("a skipped source-update step must explain itself")
			}
		})
	}
}

// TestSourceUpdatePromptSubjectIsTheReusedBranch: when the worktree's branch
// already exists locally, its own commits are what gets checked out — so the
// fast-forward targets that branch, never the recorded parent.
func TestSourceUpdatePromptSubjectIsTheReusedBranch(t *testing.T) {
	work := repoWithRemote(t)
	behindBranch(t, work, "feat/reused")

	got := promptFor(t, work, "feat/reused", "main")
	if !got.Show {
		t.Fatalf("prompt = %+v, want an offer on the reused branch", got)
	}
	if got.Branch != "feat/reused" {
		t.Errorf("Branch = %q, want the reused branch, not the parent", got.Branch)
	}
}

// TestFFSubjectFollowsTheBranchTheWorktreeStartsFrom is the non-interactive
// counterpart of the wizard's subject choice.
func TestFFSubjectFollowsTheBranchTheWorktreeStartsFrom(t *testing.T) {
	tests := []struct {
		name  string
		state domain.BranchTargetState
		want  string
	}{
		{"a new branch starts from its source", domain.BranchTargetNew, "main"},
		{"an existing branch is its own start-point", domain.BranchTargetExisting, "feat/x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ffSubject(ffSubjectParams{
				Target:     domain.BranchTarget{State: tt.state},
				FromBranch: "main",
				Branch:     "feat/x",
			})
			if got != tt.want {
				t.Errorf("ffSubject = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestCreateDisplayPathPrefersTheConfiguredLocation renders a fresh worktree as
// base_path/<name>, and keeps the real path for one living elsewhere.
func TestCreateDisplayPathPrefersTheConfiguredLocation(t *testing.T) {
	config := domain.Config{}
	config.Project.Worktrees.BasePath = "../.trees"

	inBase := createDisplayPath(displayPathParams{
		Config:     config,
		ProjectDir: "/repo",
		Path:       "/.trees/feat-x",
	})
	if inBase != "../.trees/feat-x" {
		t.Errorf("display path = %q, want the base_path form", inBase)
	}

	elsewhere := createDisplayPath(displayPathParams{
		Config:     config,
		ProjectDir: "/repo",
		Path:       "/somewhere/else/feat-x",
	})
	if elsewhere != "/somewhere/else/feat-x" {
		t.Errorf("display path = %q, want the real path for a foreign worktree", elsewhere)
	}
}

// behindBranch creates branch on main, pushes it, then advances origin so the
// local branch is behind by one commit.
func behindBranch(t *testing.T, work, name string) {
	t.Helper()
	gitRun(t, work, "branch", name)
	gitRun(t, work, "push", "origin", name)
	gitRun(t, work, "commit", "--allow-empty", "-m", "server-commit")
	gitRun(t, work, "push", "origin", "main:"+name)
	gitRun(t, work, "reset", "--hard", "HEAD~1")
	gitRun(t, work, "fetch", "origin")
}

// divergedBranch creates branch with one local commit origin does not have, while
// origin has one the local branch does not — one ahead, one behind.
func divergedBranch(t *testing.T, work, name string) {
	t.Helper()
	behindBranch(t, work, name)
	gitRun(t, work, "commit", "--allow-empty", "-m", "local-commit")
	gitRun(t, work, "branch", "-f", name, "HEAD")
	gitRun(t, work, "reset", "--hard", "HEAD~1")
}

// TestBehindAndDivergedFixtures guards the fixtures themselves: a wrong fixture
// would make the divergence assertions above pass for the wrong reason.
func TestBehindAndDivergedFixtures(t *testing.T) {
	work := repoWithRemote(t)
	behindBranch(t, work, "feat/b")
	divergedBranch(t, work, "feat/d")

	if got := revParse(t, work, "feat/b"); got == revParse(t, work, "origin/feat/b") {
		t.Error("the behind fixture should differ from its origin counterpart")
	}
	if revParse(t, work, "feat/d") == revParse(t, work, "origin/feat/d") {
		t.Error("the diverged fixture should differ from its origin counterpart")
	}
}
