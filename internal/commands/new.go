package commands

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	newpicker "github.com/LucasPcq/wtm/internal/tui/new"
)

// NewNewCmd creates the wtm new command.
func NewNewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new [branch]",
		Short: "Create a new worktree",
		Long:  "Create a git worktree with env provisioning, metadata, and hooks.\nWithout arguments, prompts for the branch name interactively.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runNew,
	}

	cmd.Flags().String(domain.FlagFrom, "", "Source branch (skips interactive picker)")
	cmd.Flags().String(domain.FlagEnvFrom, "", "Override env strategy (example, main, parent)")

	return cmd
}

func runNew(cmd *cobra.Command, args []string) error {
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

	result, ok := loadConfig(cmd, dir)
	if !ok {
		return nil
	}

	if branch == "" {
		prompted, promptErr := promptBranchName()
		if errors.Is(promptErr, domain.ErrUserAborted) {
			return nil
		}
		if promptErr != nil {
			return promptErr
		}
		branch = prompted
	}

	fromBranch := fromFlag
	envOverride := envFromFlag

	if fromFlag == "" {
		wizResult, wizErr := runNewWizard(result.ProjectDir, result.Config)
		if errors.Is(wizErr, domain.ErrUserAborted) {
			return nil
		}
		if wizErr != nil {
			return wizErr
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
		if !branchInList(branches, fromFlag) {
			return fmt.Errorf("%w: %s", domain.ErrBranchNotFound, fromFlag)
		}
	}

	createResult, err := worktree.Create(worktree.CreateParams{
		ProjectDir:      result.ProjectDir,
		Branch:          branch,
		FromBranch:      fromBranch,
		Config:          result.Config,
		EnvFromOverride: envOverride,
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Created worktree %s at %s\n", branch, createResult.Path)

	return nil
}

func runNewWizard(projectDir string, cfg domain.Config) (newpicker.WizardResult, error) {
	branches, err := infra.ListLocalBranches(infra.ListBranchesParams{ProjectDir: projectDir})
	if err != nil {
		return newpicker.WizardResult{}, fmt.Errorf("list branches: %w", err)
	}

	return newpicker.RunWizard(newpicker.WizardParams{
		Branches:       branches,
		DefaultBranch:  cfg.Project.Worktrees.BaseBranch,
		ConfigStrategy: cfg.Project.Env.Strategy,
	})
}

func promptBranchName() (string, error) {
	var name string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Branch name").
				Description("Name for the new worktree branch").
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("branch name is required")
					}
					return nil
				}).
				Value(&name),
		),
	)

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", domain.ErrUserAborted
		}
		return "", err
	}

	return strings.TrimSpace(name), nil
}

func branchInList(branches []string, target string) bool {
	for _, b := range branches {
		if b == target {
			return true
		}
	}
	return false
}
