package infra

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitCommonDirParams holds inputs for resolving the git common dir.
type GitCommonDirParams struct {
	Dir string
}

// GitCommonDir runs `git rev-parse --git-common-dir` and resolves the result
// to an absolute path. The common dir is shared across all worktrees of a
// clone, so the returned path is stable from any worktree.
func GitCommonDir(params GitCommonDirParams) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--git-common-dir")
	cmd.Dir = params.Dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --git-common-dir: %w", err)
	}

	path := strings.TrimSpace(string(out))
	if !filepath.IsAbs(path) {
		path = filepath.Join(params.Dir, path)
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}
	return abs, nil
}
