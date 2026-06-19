package wt

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	extracttui "github.com/LucasPcq/wtm/internal/tui/extract"
	newpicker "github.com/LucasPcq/wtm/internal/tui/newwt"
)

// newExtractCmd creates the wtm wt extract subcommand.
func newExtractCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdExtract,
		Short: "Move uncommitted changes to another worktree",
		Long: "Move a subset of the current worktree's uncommitted changes to another worktree\n" +
			"(new or existing) to split an oversized PR or isolate unrelated work.\n" +
			"The extraction is transactional: on conflict it aborts, leaving the source intact.",
		RunE: runExtract,
	}

	cmd.Flags().StringSlice(domain.FlagFiles, nil, "Files to extract (skips interactive selection)")
	cmd.Flags().String(domain.FlagTo, "", "Target worktree branch; created if it does not exist")
	cmd.Flags().String(domain.FlagFrom, "", "Parent branch when creating the target worktree")
	cmd.Flags().Bool(domain.FlagKeep, false, "Copy instead of move (keep the changes in the source)")
	shared.AddOutputFlag(cmd)

	return cmd
}

func runExtract(cmd *cobra.Command, _ []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	sourcePath, err := infra.Toplevel(dir)
	if err != nil {
		return err
	}
	sourceBranch, err := infra.CurrentBranch(sourcePath)
	if err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	available, err := listExtractFiles(sourcePath)
	if err != nil {
		return err
	}
	if len(available) == 0 {
		if format == domain.OutputJSON {
			return output.WriteExtractJSON(cmd.OutOrStdout(), domain.ExtractResult{Files: []domain.ExtractFile{}})
		}
		output.Blank(cmd.OutOrStdout())
		output.Message(cmd.OutOrStdout(), domain.ErrNoChangesToExtract.Error())
		output.Blank(cmd.OutOrStdout())
		return nil
	}

	selected, target, err := resolveSelectionAndTarget(resolveParams{
		cmd:          cmd,
		cfg:          cfg,
		sourceBranch: sourceBranch,
		available:    available,
	})
	if errors.Is(err, domain.ErrUserAborted) {
		return nil
	}
	if err != nil {
		return err
	}

	keep, _ := cmd.Flags().GetBool(domain.FlagKeep)
	result, err := worktree.Extract(domain.ExtractParams{
		SourcePath:   sourcePath,
		TargetPath:   target.path,
		TargetBranch: target.branch,
		Files:        selected,
		Keep:         keep,
	})
	if err != nil {
		return err
	}

	if format == domain.OutputJSON {
		return output.WriteExtractJSON(cmd.OutOrStdout(), result)
	}
	output.PrintExtractResult(cmd.OutOrStdout(), result)
	return nil
}

// listExtractFiles returns the uncommitted files of a worktree classified for
// extraction.
func listExtractFiles(sourcePath string) ([]domain.ExtractFile, error) {
	modified, err := infra.ListModifiedFiles(infra.ListModifiedFilesParams{WorktreePath: sourcePath})
	if err != nil {
		return nil, fmt.Errorf("list modified files: %w", err)
	}

	files := make([]domain.ExtractFile, 0, len(modified))
	for _, m := range modified {
		files = append(files, domain.ExtractFile{
			Path:   m.Path,
			Status: rules.ClassifyExtractStatus(m.Status),
		})
	}
	return files, nil
}

type resolveParams struct {
	cmd          *cobra.Command
	cfg          shared.ConfigResult
	sourceBranch string
	available    []domain.ExtractFile
}

// resolveSelectionAndTarget resolves the files to extract and the destination
// worktree, from flags or the interactive wizard. When the target is chosen
// interactively, it loops so that backing out of the new-worktree sub-flow
// returns to the Target step with the file selection preserved.
func resolveSelectionAndTarget(p resolveParams) ([]domain.ExtractFile, extractTarget, error) {
	filesFlag, _ := p.cmd.Flags().GetStringSlice(domain.FlagFiles)
	toFlag, _ := p.cmd.Flags().GetString(domain.FlagTo)
	needFiles := len(filesFlag) == 0
	needTarget := toFlag == ""

	preselected := filesFlag
	startAtTarget := false

	for {
		wizard, err := runWizard(runWizardParams{
			cfg:           p.cfg,
			available:     p.available,
			sourceBranch:  p.sourceBranch,
			needFiles:     needFiles,
			needTarget:    needTarget,
			preselected:   preselected,
			startAtTarget: startAtTarget,
		})
		if err != nil {
			return nil, extractTarget{}, err
		}

		selectedPaths := filesFlag
		if needFiles {
			selectedPaths = wizard.Files
			preselected = wizard.Files
		}
		selected, err := filterByPaths(p.available, selectedPaths)
		if err != nil {
			return nil, extractTarget{}, err
		}

		target, err := resolveTarget(resolveTargetParams{
			cmd:          p.cmd,
			cfg:          p.cfg,
			sourceBranch: p.sourceBranch,
			choice:       wizard.Target,
		})
		if needTarget && errors.Is(err, domain.ErrUserAborted) {
			// Backed out of the new-worktree sub-flow: re-enter at the Target step.
			startAtTarget = true
			continue
		}
		if err != nil {
			return nil, extractTarget{}, err
		}
		return selected, target, nil
	}
}

type runWizardParams struct {
	cfg           shared.ConfigResult
	available     []domain.ExtractFile
	sourceBranch  string
	needFiles     bool
	needTarget    bool
	preselected   []string
	startAtTarget bool
}

// runWizard runs the unified file+target selection wizard, skipping any step
// already resolved from a flag. Returns a zero result when nothing is needed.
func runWizard(params runWizardParams) (extracttui.RunResult, error) {
	if !params.needFiles && !params.needTarget {
		return extracttui.RunResult{}, nil
	}

	var statuses []domain.WorktreeStatus
	if params.needTarget {
		var err error
		statuses, err = worktree.List(domain.ListParams{
			ProjectDir: params.cfg.ProjectDir,
			StateDir:   params.cfg.StateDir,
			Config:     params.cfg.Config,
		})
		if err != nil {
			return extracttui.RunResult{}, fmt.Errorf("list worktrees: %w", err)
		}
	}

	return extracttui.Run(extracttui.RunParams{
		Files:         params.available,
		Worktrees:     statuses,
		SourceBranch:  params.sourceBranch,
		NeedFiles:     params.needFiles,
		NeedTarget:    params.needTarget,
		Preselected:   params.preselected,
		StartAtTarget: params.startAtTarget,
	})
}

// filterByPaths returns the available files whose path is in paths, erroring on
// any path that is not a current uncommitted change.
func filterByPaths(available []domain.ExtractFile, paths []string) ([]domain.ExtractFile, error) {
	byPath := make(map[string]domain.ExtractFile, len(available))
	for _, f := range available {
		byPath[f.Path] = f
	}

	out := make([]domain.ExtractFile, 0, len(paths))
	for _, p := range paths {
		f, ok := byPath[p]
		if !ok {
			return nil, fmt.Errorf("%w: %s is not an uncommitted change", domain.ErrNoFilesSelected, p)
		}
		out = append(out, f)
	}
	if len(out) == 0 {
		return nil, domain.ErrNoFilesSelected
	}
	return out, nil
}

type extractTarget struct {
	path   string
	branch string
}

type resolveTargetParams struct {
	cmd          *cobra.Command
	cfg          shared.ConfigResult
	sourceBranch string
	choice       extracttui.TargetChoice
}

// resolveTarget resolves the destination worktree from the --to flag or the
// wizard choice, creating a new worktree when requested.
func resolveTarget(params resolveTargetParams) (extractTarget, error) {
	toFlag, _ := params.cmd.Flags().GetString(domain.FlagTo)
	if toFlag != "" {
		if wt, err := infra.FindWorktreeByBranch(infra.FindWorktreeByBranchParams{
			ProjectDir: params.cfg.ProjectDir,
			Branch:     toFlag,
		}); err == nil {
			return extractTarget{path: wt.Path, branch: wt.Branch}, nil
		}
		fromFlag, _ := params.cmd.Flags().GetString(domain.FlagFrom)
		return createTarget(params.cfg, toFlag, defaultParent(params.cfg, params.sourceBranch, fromFlag))
	}

	if !params.choice.CreateNew {
		wt, findErr := infra.FindWorktreeByBranch(infra.FindWorktreeByBranchParams{
			ProjectDir: params.cfg.ProjectDir,
			Branch:     params.choice.Branch,
		})
		if findErr != nil {
			return extractTarget{}, findErr
		}
		return extractTarget{path: wt.Path, branch: wt.Branch}, nil
	}

	return createTargetInteractive(params)
}

// createTargetInteractive runs the new-worktree wizard with the source's parent
// branch pre-selected, then creates the worktree.
func createTargetInteractive(params resolveTargetParams) (extractTarget, error) {
	branches, err := infra.ListLocalBranches(infra.ListBranchesParams{ProjectDir: params.cfg.ProjectDir})
	if err != nil {
		return extractTarget{}, fmt.Errorf("list branches: %w", err)
	}

	wiz, err := newpicker.RunWizard(newpicker.WizardParams{
		Branches:       branches,
		DefaultBranch:  defaultParent(params.cfg, params.sourceBranch, ""),
		ConfigStrategy: params.cfg.Config.Project.Env.Strategy,
		IncludeBranch:  true,
	})
	if err != nil {
		return extractTarget{}, err
	}

	return createTarget(params.cfg, wiz.BranchName, wiz.FromBranch)
}

// defaultParent resolves the parent branch for a new target worktree: the
// explicit override, else the source worktree's recorded parent, else the
// configured base branch.
func defaultParent(cfg shared.ConfigResult, sourceBranch, override string) string {
	if override != "" {
		return override
	}
	if parent := worktree.ParentBranch(worktree.ParentBranchParams{
		StateDir: cfg.StateDir,
		Branch:   sourceBranch,
	}); parent != "" {
		return parent
	}
	return cfg.Config.Project.Worktrees.BaseBranch
}

func createTarget(cfg shared.ConfigResult, branch, fromBranch string) (extractTarget, error) {
	res, err := worktree.Create(domain.CreateParams{
		ProjectDir: cfg.ProjectDir,
		StateDir:   cfg.StateDir,
		Branch:     branch,
		FromBranch: fromBranch,
		Config:     cfg.Config,
	})
	if err != nil {
		return extractTarget{}, err
	}
	return extractTarget{path: res.Path, branch: res.Branch}, nil
}
