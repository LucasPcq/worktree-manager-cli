package wt

import (
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	newpicker "github.com/LucasPcq/wtm/internal/tui/newwt"
)

// newCreateCmd creates the wtm create subcommand.
func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdCreate + " [branch]",
		Short: "Create a new worktree",
		Long:  "Create a git worktree with env provisioning, metadata, and hooks.\nWithout arguments, prompts for the branch name interactively.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runCreate,
	}

	cmd.Flags().String(domain.FlagFrom, "", "Source branch (skips interactive picker)")
	cmd.Flags().String(domain.FlagEnvFrom, "", "Override env strategy (example, main, parent)")
	cmd.Flags().Bool(domain.FlagIfNotExists, false, "Succeed silently if the worktree already exists (idempotent)")
	shared.AddOutputFlag(cmd)

	return cmd
}

func runCreate(cmd *cobra.Command, args []string) error {
	var branch string
	if len(args) > 0 {
		branch = args[0]
	}
	fromFlag, _ := cmd.Flags().GetString(domain.FlagFrom)
	envFromFlag, _ := cmd.Flags().GetString(domain.FlagEnvFrom)
	ifNotExists, _ := cmd.Flags().GetBool(domain.FlagIfNotExists)

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	fromBranch := fromFlag
	envOverride := envFromFlag

	if fromFlag == "" {
		wizResult, wizErr := runCreateWizard(createWizardParams{
			ProjectDir:    result.ProjectDir,
			Config:        result.Config,
			IncludeBranch: branch == "",
		})
		if errors.Is(wizErr, domain.ErrUserAborted) {
			return nil
		}
		if wizErr != nil {
			return wizErr
		}
		if branch == "" {
			branch = wizResult.BranchName
		}
		fromBranch = wizResult.FromBranch
		if envFromFlag == "" {
			envOverride = wizResult.EnvFromOverride
		}
	} else {
		branches, listErr := infra.ListLocalBranches(infra.ListBranchesParams{ProjectDir: result.ProjectDir})
		if listErr != nil {
			return fmt.Errorf("list branches: %w", listErr)
		}
		if !slices.Contains(branches, fromFlag) {
			return fmt.Errorf("%w: %s", domain.ErrBranchNotFound, fromFlag)
		}
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	// on_create hooks stream their own output, so only show a spinner for the
	// silent path (git worktree add + env copy) on the human-facing run.
	hasHooks := len(result.Config.Project.Hooks.OnCreate) > 0
	showSpinner := rules.IsHumanFormat(format) && !hasHooks
	var stop func()
	if showSpinner {
		stop = shared.StartSpinner(cmd.ErrOrStderr(), "Creating worktree…")
	}

	createResult, err := worktree.Create(domain.CreateParams{
		ProjectDir:      result.ProjectDir,
		StateDir:        result.StateDir,
		Branch:          branch,
		FromBranch:      fromBranch,
		Config:          result.Config,
		EnvFromOverride: envOverride,
		IfNotExists:     ifNotExists,
	})
	if stop != nil {
		stop()
	}
	if err != nil {
		return err
	}

	if format == domain.OutputJSON {
		return output.WriteWorktreeCreateJSON(cmd.OutOrStdout(), createResult)
	}

	output.Frame(cmd.OutOrStdout(), func() {
		if createResult.AlreadyExists {
			output.Success(cmd.OutOrStdout(), fmt.Sprintf("Worktree %s already exists at %s", branch, createResult.Path))
		} else {
			output.Success(cmd.OutOrStdout(), fmt.Sprintf("Created worktree %s at %s", branch, createResult.Path))
		}
	})

	return nil
}

type createWizardParams struct {
	ProjectDir    string
	Config        domain.Config
	IncludeBranch bool
}

func runCreateWizard(params createWizardParams) (newpicker.WizardResult, error) {
	branches, err := infra.ListLocalBranches(infra.ListBranchesParams{ProjectDir: params.ProjectDir})
	if err != nil {
		return newpicker.WizardResult{}, fmt.Errorf("list branches: %w", err)
	}

	return newpicker.RunWizard(newpicker.WizardParams{
		Branches:       branches,
		DefaultBranch:  params.Config.Project.Worktrees.BaseBranch,
		ConfigStrategy: params.Config.Project.Env.Strategy,
		IncludeBranch:  params.IncludeBranch,
	})
}
