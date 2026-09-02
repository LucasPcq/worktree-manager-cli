package run

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/tui/runpicker"
)

// pickWorktree is a variable so a test can answer for the picker without a
// terminal, the way showRunView stands in for the full-screen view.
var pickWorktree = runpicker.RunWorktreePicker

type targetParams struct {
	// Args is the command's positional slice; its first entry, when present,
	// names the worktree.
	Args        []string
	Cwd         string
	ProjectDir  string
	Interactive bool
	// Pick allows the picker. `run url` sets it false: it refuses a picker on
	// its job axis too, and opening one inside $(…) would hang the shell.
	Pick bool
}

type target struct {
	Dir    string
	Branch string
}

// resolveTarget answers which worktree a run command acts on. The worktree is a
// decision with a safe default — the current directory — so the non-interactive
// path never errors for want of it and never opens a picker for it.
func resolveTarget(params targetParams) (target, error) {
	if query := firstArg(params.Args); query != "" {
		return resolveNamed(params.ProjectDir, query)
	}
	if !params.Interactive || !params.Pick {
		return target{Dir: params.Cwd}, nil
	}
	return pickTarget(params)
}

func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

func resolveNamed(projectDir, query string) (target, error) {
	result, err := worktree.Resolve(domain.ResolveParams{ProjectDir: projectDir, Query: query})
	if err != nil {
		return target{}, fmt.Errorf("worktree %q: %w", query, err)
	}
	if result.Ambiguous {
		return target{}, fmt.Errorf("worktree %q is ambiguous: matches %s", query, branchNames(result.Matches))
	}
	return target{Dir: result.Path, Branch: result.Branch}, nil
}

func pickTarget(params targetParams) (target, error) {
	worktrees, err := worktree.ListAll(worktree.ListAllParams{ProjectDir: params.ProjectDir})
	if err != nil {
		return target{}, fmt.Errorf("list worktrees: %w", err)
	}
	if len(worktrees) <= 1 {
		return target{Dir: params.Cwd}, nil
	}

	picked, err := pickWorktree(runpicker.WorktreePickerParams{
		Worktrees: worktrees,
		Current:   currentPath(worktrees, params.Cwd),
		Running:   rules.RunningJobsByWorktree(shared.LoadJobsGraceful()),
	})
	if err != nil {
		return target{}, err
	}
	return target{Dir: picked.Path, Branch: picked.Branch}, nil
}

// currentPath maps the directory the command was launched from onto the worktree
// containing it, which may be one of its subdirectories. Both sides go through
// infra.ResolvePath: on macOS the cwd is reached through /var and git reports
// /private/var, and a raw compare would leave the cursor on the wrong entry. An
// unmatched cwd pre-selects nothing rather than the wrong worktree.
func currentPath(worktrees []domain.GitWorktree, cwd string) string {
	resolved := infra.ResolvePath(cwd)
	for _, wt := range worktrees {
		path := infra.ResolvePath(wt.Path)
		if resolved == path || strings.HasPrefix(resolved, path+string(filepath.Separator)) {
			return wt.Path
		}
	}
	return ""
}

// pickJob is a variable for the same reason as pickWorktree.
var pickJob = runpicker.RunJobPicker

type jobParams struct {
	Name        string
	Config      domain.RunConfig
	Interactive bool
}

// resolveJob answers which job `run start` / `run stop` acts on. Unlike the
// worktree, the job has no safe default: without --job the non-interactive path
// names the flag rather than guessing or falling back to a picker.
func resolveJob(params jobParams) (domain.JobConfig, error) {
	if params.Name != "" {
		job, ok := rules.FindJob(params.Config, params.Name)
		if !ok {
			return domain.JobConfig{}, fmt.Errorf("%w: %s", domain.ErrJobNotFound, params.Name)
		}
		return job, nil
	}
	if !params.Interactive {
		return domain.JobConfig{}, domain.ErrJobRequired
	}
	if len(params.Config.Jobs) == 0 {
		return domain.JobConfig{}, domain.ErrJobRequired
	}
	return pickJob(params.Config.Jobs)
}

func branchNames(worktrees []domain.GitWorktree) string {
	names := make([]string, 0, len(worktrees))
	for _, wt := range worktrees {
		names = append(names, wt.Branch)
	}
	return strings.Join(names, domain.RunURLListSep)
}
