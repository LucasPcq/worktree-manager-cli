package commands

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	ghservice "github.com/LucasPcq/wtm/internal/service/github"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	prwizard "github.com/LucasPcq/wtm/internal/tui/pr"
)

func newPRCheckoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "checkout [number]",
		Short: "Create a worktree from an existing pull request",
		Long:  "Create a worktree from a pull request.\nWithout arguments, shows an interactive picker of open PRs.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runPRCheckout,
	}

	cmd.Flags().String(domain.FlagEnvFrom, "", "Override env strategy (example, main, parent)")

	return cmd
}

func runPRCheckout(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, ok := loadConfig(cmd, dir)
	if !ok {
		return nil
	}

	number, err := resolvePRNumberArg(cmd, result.ProjectDir, args)
	if err != nil {
		return err
	}
	if number == 0 {
		return nil
	}

	envFromOverride, _ := cmd.Flags().GetString(domain.FlagEnvFrom)

	return checkoutPR(cmd, result, checkoutPRParams{
		Number:          number,
		EnvFromOverride: envFromOverride,
	})
}

// resolvePRNumberArg parses the positional number argument, or opens an
// interactive PR picker when no argument is provided.
// Returns (0, nil) when the user aborts the picker.
func resolvePRNumberArg(cmd *cobra.Command, projectDir string, args []string) (int, error) {
	if len(args) == 1 {
		number, err := strconv.Atoi(args[0])
		if err != nil || number <= 0 {
			return 0, fmt.Errorf("invalid PR number %q", args[0])
		}
		return number, nil
	}

	return pickPRNumber(cmd, projectDir)
}

// checkoutPRParams holds inputs for the shared PR checkout logic.
type checkoutPRParams struct {
	Number          int
	EnvFromOverride string // empty triggers the interactive env wizard
}

// checkoutPR is the shared implementation used by `wtm pr checkout`,
// the `wtm pr list` picker action, and the dashboard keybinding.
func checkoutPR(cmd *cobra.Command, result configResult, params checkoutPRParams) error {
	if _, err := ghservice.ResolveAuth(); err != nil {
		if errors.Is(err, domain.ErrAuthNotConfigured) || errors.Is(err, domain.ErrAuthNeedsReauth) {
			fmt.Fprintln(cmd.ErrOrStderr(), err.Error())
			return nil
		}
		return fmt.Errorf("load auth: %w", err)
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Fetching PR #%d...\n", params.Number)

	pr, err := ghservice.GetPRDetail(ghservice.GetPRDetailParams{
		ProjectDir: result.ProjectDir,
		Number:     params.Number,
	})
	if err != nil {
		return fmt.Errorf("fetch PR: %w", err)
	}

	localBranches, err := infra.ListLocalBranches(infra.ListBranchesParams{
		ProjectDir: result.ProjectDir,
	})
	if err != nil {
		return fmt.Errorf("list local branches: %w", err)
	}

	if err := validatePRForCheckout(pr, localBranches); err != nil {
		return err
	}

	envFromOverride := params.EnvFromOverride
	if envFromOverride == "" {
		picked, pickErr := prwizard.RunEnvPicker(prwizard.EnvPickerParams{
			ConfigStrategy: result.Config.Project.Env.Strategy,
		})
		if pickErr != nil {
			if errors.Is(pickErr, domain.ErrUserAborted) {
				return nil
			}
			return pickErr
		}
		envFromOverride = picked
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Fetching branch %s from origin...\n", pr.Branch)
	if err := infra.FetchBranch(infra.FetchBranchParams{
		ProjectDir: result.ProjectDir,
		Branch:     pr.Branch,
	}); err != nil {
		return err
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Creating worktree %s...\n", pr.Branch)
	createResult, err := worktree.Create(worktree.CreateParams{
		ProjectDir:      result.ProjectDir,
		Branch:          pr.Branch,
		FromBranch:      "origin/" + pr.Branch,
		Config:          result.Config,
		EnvFromOverride: envFromOverride,
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\n✓ Checked out PR #%d (%s) at %s\n", pr.Number, pr.Branch, createResult.Path)
	fmt.Fprintf(cmd.OutOrStdout(), "  wtm wt go %s\n", pr.Branch)
	return nil
}

// validatePRForCheckout performs the pure pre-flight checks.
// Pulled out so it can be unit-tested without hitting the API or the filesystem.
func validatePRForCheckout(pr domain.PRInfo, localBranches []string) error {
	if pr.IsFork {
		return fmt.Errorf("PR #%d is from a fork — fork support is tracked in LUC-40", pr.Number)
	}
	if slices.Contains(localBranches, pr.Branch) {
		return fmt.Errorf("local branch %q already exists — run `wtm wt clean %s` first", pr.Branch, pr.Branch)
	}
	return nil
}
