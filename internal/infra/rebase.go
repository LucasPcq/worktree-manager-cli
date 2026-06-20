package infra

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// TipParams holds inputs for resolving a ref to a short SHA.
type TipParams struct {
	WorktreePath string
	Ref          string
}

// Tip returns the short commit SHA a ref points to.
func Tip(params TipParams) (string, error) {
	cmd := exec.Command("git", "-C", params.WorktreePath, "rev-parse", "--short", params.Ref)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse %s: %w", params.Ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// RebaseResult reports the outcome of a rebase attempt.
type RebaseResult struct {
	Conflicted bool
	Output     string
}

// RebaseOntoParams holds inputs for a transplanting rebase.
type RebaseOntoParams struct {
	WorktreePath string
	NewBase      string
	Upstream     string
	Branch       string
}

// RebaseOnto runs `git rebase --onto <NewBase> <Upstream> <Branch>`, replaying
// only the commits unique to Branch (Upstream..Branch) onto NewBase. This avoids
// re-applying the parent's already-moved commits.
//
// On a conflict the rebase is aborted so the worktree is left clean, and the
// result reports Conflicted=true with a nil error. A non-conflict failure
// returns an error (after a best-effort abort).
func RebaseOnto(params RebaseOntoParams) (RebaseResult, error) {
	cmd := exec.Command("git", "-C", params.WorktreePath,
		"rebase", "--onto", params.NewBase, params.Upstream, params.Branch)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return RebaseResult{}, nil
	}

	output := strings.TrimSpace(string(out))
	abortRebase(params.WorktreePath)

	if strings.Contains(output, "CONFLICT") {
		return RebaseResult{Conflicted: true, Output: output}, nil
	}
	return RebaseResult{Output: output}, fmt.Errorf("git rebase: %s", output)
}

// abortRebase best-effort aborts an in-progress rebase. A "no rebase in
// progress" error is expected when the failure happened before any apply.
func abortRebase(worktreePath string) {
	exec.Command("git", "-C", worktreePath, "rebase", "--abort").Run()
}

// FastForwardParams holds inputs for fast-forwarding a checked-out branch.
type FastForwardParams struct {
	WorktreePath string
	Onto         string
}

// FastForwardBranch fast-forwards the branch checked out in WorktreePath to Onto
// (e.g. origin/main). It fails if the branch has diverged (no fast-forward).
func FastForwardBranch(params FastForwardParams) error {
	cmd := exec.Command("git", "-C", params.WorktreePath, "merge", "--ff-only", params.Onto)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git merge --ff-only %s: %s", params.Onto, strings.TrimSpace(string(out)))
	}
	return nil
}

// BehindParams holds inputs for counting how far a branch is behind an upstream.
type BehindParams struct {
	WorktreePath string
	Branch       string
	Upstream     string
}

// Behind returns the number of commits present in Upstream but not in Branch.
// Zero means the branch already contains everything from its parent.
func Behind(params BehindParams) (int, error) {
	return revListCount(params.WorktreePath, params.Branch+".."+params.Upstream)
}

// CommitCountParams holds inputs for counting commits in a revision range.
type CommitCountParams struct {
	WorktreePath string
	Range        string
}

// CommitCount returns the number of commits in the given revision range
// (e.g. "<oldParentTip>..<branch>").
func CommitCount(params CommitCountParams) (int, error) {
	return revListCount(params.WorktreePath, params.Range)
}

func revListCount(worktreePath, revRange string) (int, error) {
	cmd := exec.Command("git", "-C", worktreePath, "rev-list", "--count", revRange)
	out, err := cmd.Output()
	if err != nil {
		return 0, nil
	}
	count, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, nil
	}
	return count, nil
}

// IsAncestorParams holds inputs for an ancestry check.
type IsAncestorParams struct {
	WorktreePath string
	Ancestor     string
	Descendant   string
}

// IsAncestor reports whether Ancestor is an ancestor of (or equal to) Descendant.
func IsAncestor(params IsAncestorParams) bool {
	cmd := exec.Command("git", "-C", params.WorktreePath,
		"merge-base", "--is-ancestor", params.Ancestor, params.Descendant)
	return cmd.Run() == nil
}

// PushForceParams holds inputs for a force-with-lease push.
type PushForceParams struct {
	WorktreePath string
	Branch       string
}

// PushForceWithLease pushes Branch to origin with --force-with-lease, which
// refuses to overwrite remote commits the local ref has not seen.
func PushForceWithLease(params PushForceParams) error {
	cmd := exec.Command("git", "-C", params.WorktreePath,
		"push", "--force-with-lease", "origin", params.Branch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git push --force-with-lease origin %s: %s", params.Branch, strings.TrimSpace(string(out)))
	}
	return nil
}
