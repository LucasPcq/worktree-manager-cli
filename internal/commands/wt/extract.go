package wt

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/branch"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	extracttui "github.com/LucasPcq/wtm/internal/tui/extract"
	newpicker "github.com/LucasPcq/wtm/internal/tui/newwt"
)

// newExtractCmd creates the wtm extract subcommand.
func newExtractCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdExtract,
		Short: "Move uncommitted changes to another worktree",
		Long: "Move a subset of the current worktree's uncommitted changes to another worktree\n" +
			"(new or existing) to split an oversized PR or isolate unrelated work.\n" +
			"On conflict it aborts by default, leaving the source intact; --on-conflict resolve\n" +
			"applies conflict markers in the target so you can resolve them like a rebase.",
		RunE: runExtract,
	}

	cmd.Flags().StringSlice(domain.FlagFiles, nil, "Files to extract (skips interactive selection)")
	cmd.Flags().String(domain.FlagTo, "", "Target worktree branch; created if it does not exist")
	cmd.Flags().String(domain.FlagFrom, "", "Parent branch when creating the target worktree")
	cmd.Flags().Bool(domain.FlagFF, false, "Fast-forward the parent branch to origin before creating the target (non-interactive; skipped when it has diverged)")
	cmd.Flags().Bool(domain.FlagKeep, false, "Copy instead of move (keep the changes in the source)")
	cmd.Flags().String(domain.FlagOnConflict, "", "On conflict: abort (default) or resolve (write conflict markers in the target)")
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

	if err := validateOnConflict(cmd); err != nil {
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
		output.Frame(cmd.OutOrStdout(), func() {
			output.Message(cmd.OutOrStdout(), domain.ErrNoChangesToExtract.Error())
		})
		return nil
	}

	selected, target, keep, err := resolveSelectionAndTarget(resolveParams{
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

	conflictMode, err := resolveConflictMode(resolveConflictModeParams{
		cmd:        cmd,
		sourcePath: sourcePath,
		target:     target,
		selected:   selected,
	})
	if errors.Is(err, domain.ErrUserAborted) {
		if rules.IsHumanFormat(format) {
			output.Frame(cmd.OutOrStdout(), func() {
				output.Message(cmd.OutOrStdout(), "Cancelled — nothing was changed.")
			})
		}
		return nil
	}
	if err != nil {
		return err
	}

	result, err := worktree.Extract(domain.ExtractParams{
		SourcePath:   sourcePath,
		SourceBranch: sourceBranch,
		TargetPath:   target.path,
		TargetBranch: target.branch,
		Files:        selected,
		Keep:         keep,
		ConflictMode: conflictMode,
	})
	if err != nil {
		return err
	}

	if format == domain.OutputJSON {
		return output.WriteExtractJSON(cmd.OutOrStdout(), result)
	}
	if len(result.Conflicts) > 0 {
		output.Frame(cmd.OutOrStdout(), func() {
			output.PrintExtractConflicts(cmd.OutOrStdout(), result)
		})
		return nil
	}
	output.Frame(cmd.OutOrStdout(), func() {
		output.PrintExtractResult(cmd.OutOrStdout(), result)
	})
	return nil
}

type resolveConflictModeParams struct {
	cmd        *cobra.Command
	sourcePath string
	target     extractTarget
	selected   []domain.ExtractFile
}

// resolveConflictMode decides what Extract does on conflict. No conflict → abort
// (unused). On conflict: honor --on-conflict, else prompt on a TTY, else default
// to abort. Returns ErrUserAborted when the user declines the prompt.
func resolveConflictMode(p resolveConflictModeParams) (string, error) {
	conflicts := worktree.ConflictingFiles(domain.ConflictCheckParams{
		SourcePath: p.sourcePath,
		TargetPath: p.target.path,
		Files:      p.selected,
	})
	if len(conflicts) == 0 {
		return domain.OnConflictAbort, nil
	}

	// The flag value is validated at command entry, so it is safe to trust here.
	if p.cmd.Flags().Changed(domain.FlagOnConflict) {
		mode, _ := p.cmd.Flags().GetString(domain.FlagOnConflict)
		return mode, nil
	}

	if !isInteractive() {
		return domain.OnConflictAbort, nil
	}

	resolve, err := extracttui.ConfirmResolve(conflicts, p.target.branch)
	if err != nil {
		return "", err
	}
	if !resolve {
		return "", domain.ErrUserAborted
	}
	return domain.OnConflictResolve, nil
}

func isInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// validateOnConflict rejects an invalid --on-conflict value at the command
// boundary, regardless of whether the extraction ends up conflicting.
func validateOnConflict(cmd *cobra.Command) error {
	if !cmd.Flags().Changed(domain.FlagOnConflict) {
		return nil
	}
	mode, _ := cmd.Flags().GetString(domain.FlagOnConflict)
	if mode != domain.OnConflictAbort && mode != domain.OnConflictResolve {
		return fmt.Errorf("invalid --%s value %q: use %s or %s",
			domain.FlagOnConflict, mode, domain.OnConflictAbort, domain.OnConflictResolve)
	}
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
func resolveSelectionAndTarget(p resolveParams) ([]domain.ExtractFile, extractTarget, bool, error) {
	filesFlag, _ := p.cmd.Flags().GetStringSlice(domain.FlagFiles)
	toFlag, _ := p.cmd.Flags().GetString(domain.FlagTo)
	keepFlag, _ := p.cmd.Flags().GetBool(domain.FlagKeep)
	needFiles := len(filesFlag) == 0
	needTarget := toFlag == ""
	needMode := !p.cmd.Flags().Changed(domain.FlagKeep) && isInteractive()

	preselected := filesFlag
	startAtTarget := false

	for {
		wizard, err := runWizard(runWizardParams{
			cfg:           p.cfg,
			available:     p.available,
			sourceBranch:  p.sourceBranch,
			needFiles:     needFiles,
			needTarget:    needTarget,
			needMode:      needMode,
			preselected:   preselected,
			startAtTarget: startAtTarget,
		})
		if err != nil {
			return nil, extractTarget{}, false, err
		}

		selectedPaths := filesFlag
		if needFiles {
			selectedPaths = wizard.Files
			preselected = wizard.Files
		}
		selected, err := filterByPaths(p.available, selectedPaths)
		if err != nil {
			return nil, extractTarget{}, false, err
		}

		keep := keepFlag
		if needMode {
			keep = wizard.Keep
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
			return nil, extractTarget{}, false, err
		}
		return selected, target, keep, nil
	}
}

type runWizardParams struct {
	cfg           shared.ConfigResult
	available     []domain.ExtractFile
	sourceBranch  string
	needFiles     bool
	needTarget    bool
	needMode      bool
	preselected   []string
	startAtTarget bool
}

// runWizard runs the unified file+target+mode selection wizard, skipping any
// step already resolved from a flag. Returns a zero result when nothing is needed.
func runWizard(params runWizardParams) (extracttui.RunResult, error) {
	if !params.needFiles && !params.needTarget && !params.needMode {
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
		NeedMode:      params.needMode,
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
		fromBranch := defaultParent(defaultParentParams{cfg: params.cfg, sourceBranch: params.sourceBranch, override: fromFlag})
		if isInteractive() {
			if !maybeFastForwardSource(params.cfg.ProjectDir, fromBranch) {
				return extractTarget{}, domain.ErrUserAborted
			}
		} else if ffFlag, _ := params.cmd.Flags().GetBool(domain.FlagFF); ffFlag {
			_ = branch.FastForwardIfBehind(branch.BranchParams{ProjectDir: params.cfg.ProjectDir, Branch: fromBranch})
		}
		return createTarget(createTargetParams{
			cfg:        params.cfg,
			branch:     toFlag,
			fromBranch: fromBranch,
		})
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
	wiz, err := newpicker.RunWizard(newpicker.WizardParams{
		ProjectDir:     params.cfg.ProjectDir,
		Branches:       branchCandidates(params.cfg.ProjectDir),
		DefaultBranch:  defaultParent(defaultParentParams{cfg: params.cfg, sourceBranch: params.sourceBranch}),
		ConfigStrategy: params.cfg.Config.Project.Env.Strategy,
		IncludeBranch:  true,
	})
	if err != nil {
		return extractTarget{}, err
	}

	if !maybeFastForwardSource(params.cfg.ProjectDir, wiz.FromBranch) {
		return extractTarget{}, domain.ErrUserAborted
	}
	if !shared.ConfirmEnvParentFallback(shared.EnvFallbackParams{
		ProjectDir: params.cfg.ProjectDir,
		Source:     wiz.FromBranch,
		Config:     params.cfg.Config,
	}) {
		return extractTarget{}, domain.ErrUserAborted
	}

	return createTarget(createTargetParams{cfg: params.cfg, branch: wiz.BranchName, fromBranch: wiz.FromBranch})
}

type defaultParentParams struct {
	cfg          shared.ConfigResult
	sourceBranch string
	override     string
}

// defaultParent resolves the parent branch for a new target worktree: the
// explicit override, else the source worktree's recorded parent, else the
// configured base branch.
func defaultParent(params defaultParentParams) string {
	if params.override != "" {
		return params.override
	}
	if parent := worktree.ParentBranch(worktree.ParentBranchParams{
		StateDir: params.cfg.StateDir,
		Branch:   params.sourceBranch,
	}); parent != "" {
		return parent
	}
	return params.cfg.Config.Project.Worktrees.BaseBranch
}

type createTargetParams struct {
	cfg        shared.ConfigResult
	branch     string
	fromBranch string
}

func createTarget(params createTargetParams) (extractTarget, error) {
	res, err := worktree.Create(domain.CreateParams{
		ProjectDir: params.cfg.ProjectDir,
		StateDir:   params.cfg.StateDir,
		Branch:     params.branch,
		FromBranch: params.fromBranch,
		Config:     params.cfg.Config,
	})
	if err != nil {
		return extractTarget{}, err
	}
	return extractTarget{path: res.Path, branch: res.Branch}, nil
}
