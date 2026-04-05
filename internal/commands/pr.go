package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	ghservice "github.com/LucasPcq/wtm/internal/service/github"
)

// NewPRCmd creates the wtm pr command group.
func NewPRCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pr",
		Short: "Manage pull requests",
	}

	cmd.AddCommand(newPRListCmd())

	return cmd
}

func newPRListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List open pull requests",
		RunE:  runPRList,
	}

	cmd.Flags().Bool("review", false, "Show only PRs where you are requested as reviewer")
	cmd.Flags().Bool("mine", false, "Show only your PRs")

	return cmd
}

func runPRList(cmd *cobra.Command, _ []string) error {
	dir, err := projectRootFromCwd()
	if err != nil {
		return err
	}

	auth, err := config.LoadAuth()
	if errors.Is(err, domain.ErrAuthNotConfigured) {
		fmt.Fprintln(cmd.ErrOrStderr(), "Not authenticated — run `wtm auth login` first.")
		return nil
	}
	if err != nil {
		return fmt.Errorf("load auth: %w", err)
	}

	filter := domain.PRFilterAll

	review, _ := cmd.Flags().GetBool("review")
	mine, _ := cmd.Flags().GetBool("mine")

	if review {
		filter = domain.PRFilterReviewRequested
	} else if mine {
		filter = domain.PRFilterMine
	}

	stop := startSpinner(cmd.ErrOrStderr(), "Fetching pull requests...")
	prs, err := ghservice.ListPRs(ghservice.ListPRsParams{
		ProjectDir: dir,
		Filter:     filter,
		Username:   auth.User,
	})
	stop()
	if err != nil {
		return fmt.Errorf("list PRs: %w", err)
	}

	output.PrintPRList(prs, cmd.OutOrStdout())
	return nil
}

// projectRootFromCwd resolves the project root from the current directory.
func projectRootFromCwd() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	return projectRoot(dir)
}
