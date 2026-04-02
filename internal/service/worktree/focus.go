package worktree

import (
	"fmt"
	"io"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/service/hooks"
	"github.com/LucasPcq/wtm/internal/service/state"
)

// FocusParams holds inputs for focusing a worktree.
type FocusParams struct {
	ProjectDir string
	Branch     string
	Config     domain.Config
	Output     io.Writer // if nil, hooks output goes to stderr. Set to capture (TUI mode).
}

// Focus switches the active environment to the target worktree.
// Runs on_blur hooks on the previous worktree and on_focus hooks on the new one.
func Focus(params FocusParams) error {
	wt, err := infra.FindWorktreeByBranch(infra.FindWorktreeByBranchParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
	})
	if err != nil {
		return err
	}

	current, err := state.Load()
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}

	mainPath, _ := infra.FindMainWorktreePath(infra.FindMainWorktreeParams{
		ProjectDir: params.ProjectDir,
	})

	if err := runBlurIfActive(current, params.Config, mainPath, params.Output); err != nil {
		return fmt.Errorf("on_blur: %w", err)
	}

	if len(params.Config.Project.Hooks.OnFocus) > 0 {
		if err := hooks.RunHooks(hooks.RunHooksParams{
			Hooks:   params.Config.Project.Hooks.OnFocus,
			WorkDir: wt.Path,
			Output:  params.Output,
			Vars: hooks.TemplateVars{
				Worktree: wt.Path,
				Branch:   params.Branch,
				Root:     mainPath,
			},
		}); err != nil {
			return fmt.Errorf("on_focus: %w", err)
		}
	}

	return state.Save(state.State{
		ActiveWorktree:     params.Branch,
		ActiveWorktreePath: wt.Path,
		FocusedAt:          time.Now().UTC().Format(time.RFC3339),
	})
}

// UnfocusParams holds inputs for unfocusing (--off).
type UnfocusParams struct {
	ProjectDir string
	Config     domain.Config
}

// Unfocus stops the active environment and clears the state.
func Unfocus(params UnfocusParams) error {
	current, err := state.Load()
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}

	if err := runBlurIfActive(current, params.Config, params.ProjectDir, nil); err != nil {
		return fmt.Errorf("on_blur: %w", err)
	}

	return state.Clear()
}

func runBlurIfActive(current state.State, cfg domain.Config, projectDir string, output io.Writer) error {
	if current.ActiveWorktreePath == "" {
		return nil
	}

	if len(cfg.Project.Hooks.OnBlur) == 0 {
		return nil
	}

	workDir := current.ActiveWorktreePath
	if !infra.FileExists(workDir) {
		workDir = projectDir
	}

	return hooks.RunHooks(hooks.RunHooksParams{
		Hooks:   cfg.Project.Hooks.OnBlur,
		WorkDir: workDir,
		Output:  output,
		Vars: hooks.TemplateVars{
			Worktree: current.ActiveWorktreePath,
			Branch:   current.ActiveWorktree,
			Root:     projectDir,
		},
	})
}
