package pr

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	ghservice "github.com/LucasPcq/wtm/internal/service/github"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	prwizard "github.com/LucasPcq/wtm/internal/tui/pr"
)

func newCheckoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdCheckout + " [number]",
		Short: "Create a worktree from an existing pull request",
		Long:  "Create a worktree from a pull request.\nWithout arguments, shows an interactive picker of open PRs.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runCheckout,
	}

	cmd.Flags().String(domain.FlagEnvFrom, "", "Override env strategy (example, main, parent)")
	shared.AddOutputFlag(cmd)

	return cmd
}

func runCheckout(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, ok := shared.LoadConfig(cmd, dir)
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
	EnvFromOverride string
}

// checkoutPR is the shared implementation used by runCheckout and executePRAction.
func checkoutPR(cmd *cobra.Command, result shared.ConfigResult, params checkoutPRParams) error {
	output.Loading(cmd.ErrOrStderr(), fmt.Sprintf("Fetching PR #%d...", params.Number))

	p, err := ghservice.GetPRDetail(ghservice.GetPRDetailParams{
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

	if err := rules.ValidatePRForCheckout(p, localBranches); err != nil {
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

	output.Loading(cmd.ErrOrStderr(), fmt.Sprintf("Fetching branch %s from origin...", p.Branch))
	if err := infra.FetchBranch(infra.FetchBranchParams{
		ProjectDir: result.ProjectDir,
		Branch:     p.Branch,
	}); err != nil {
		return err
	}

	output.Loading(cmd.ErrOrStderr(), fmt.Sprintf("Creating worktree %s...", p.Branch))
	createResult, err := worktree.Create(domain.CreateParams{
		ProjectDir:      result.ProjectDir,
		Branch:          p.Branch,
		FromBranch:      "origin/" + p.Branch,
		Config:          result.Config,
		EnvFromOverride: envFromOverride,
	})
	if err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	if format == domain.OutputJSON {
		return output.WritePRCheckoutJSON(cmd.OutOrStdout(), output.PRCheckoutJSON{
			Number: p.Number,
			Branch: p.Branch,
			Path:   createResult.Path,
		})
	}

	output.Blank(cmd.OutOrStdout())
	output.Success(cmd.OutOrStdout(), fmt.Sprintf("Checked out PR #%d (%s) at %s", p.Number, p.Branch, createResult.Path))
	output.InfoLine(cmd.OutOrStdout(), "cd", fmt.Sprintf("wtm wt go %s", p.Branch))
	output.Blank(cmd.OutOrStdout())
	return nil
}
