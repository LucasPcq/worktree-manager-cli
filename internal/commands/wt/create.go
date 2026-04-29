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
	"github.com/LucasPcq/wtm/internal/service/worktree"
	newpicker "github.com/LucasPcq/wtm/internal/tui/newwt"
)

// newCreateCmd creates the wtm wt create subcommand.
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

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, ok := shared.LoadConfig(cmd, dir)
	if !ok {
		return nil
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

	createResult, err := worktree.Create(domain.CreateParams{
		ProjectDir:      result.ProjectDir,
		Branch:          branch,
		FromBranch:      fromBranch,
		Config:          result.Config,
		EnvFromOverride: envOverride,
	})
	if err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	if format == domain.OutputJSON {
		return output.WriteWorktreeCreateJSON(cmd.OutOrStdout(), createResult)
	}

	output.Blank(cmd.OutOrStdout())
	output.Success(cmd.OutOrStdout(), fmt.Sprintf("Created worktree %s at %s", branch, createResult.Path))
	output.Blank(cmd.OutOrStdout())

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
