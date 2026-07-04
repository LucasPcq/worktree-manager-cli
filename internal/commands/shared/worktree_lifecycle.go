package shared

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/service/process"
)

// Worktree-removal lifecycle helpers shared by the clean and prune commands.

// RedirectToBase asks the shell wrapper to cd into the base repo, avoiding a stale
// "ghost" directory after removing the worktree we were sitting in. It relies on the
// WTM_GO_FILE bridge (see wtm shell-init); a no-op without it.
func RedirectToBase(baseDir string) {
	goFile := os.Getenv(domain.EnvGoFile)
	if goFile == "" {
		return
	}
	_ = os.WriteFile(goFile, []byte(baseDir), 0o644)
}

// ResolveSymlinks returns the canonical path, falling back to the input when it
// cannot be resolved (e.g. the path no longer exists).
func ResolveSymlinks(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

// StopWorktreeServicesParams holds inputs for StopWorktreeServices.
type StopWorktreeServicesParams struct {
	ErrOut     io.Writer
	ProjectDir string
	Branch     string
}

// StopWorktreeServices stops any running jobs for the worktree before it is removed,
// writing a confirmation to ErrOut when something was stopped.
func StopWorktreeServices(p StopWorktreeServicesParams) {
	socketPath := process.SocketPath()
	if !process.IsDaemonRunning(socketPath) {
		return
	}

	wt, err := infra.FindWorktreeByBranch(infra.FindWorktreeByBranchParams{
		ProjectDir: p.ProjectDir,
		Branch:     p.Branch,
	})
	if err != nil {
		return
	}

	client := process.NewClient(socketPath)
	if process.StopWorktreeJobs(client, wt.Path) {
		output.Success(p.ErrOut, fmt.Sprintf("Stopped services on %s", p.Branch))
	}
}
