package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	ghservice "github.com/LucasPcq/wtm/internal/service/github"
)

const (
	prActionBrowser   = "browser"
	prActionDetails   = "details"
	prActionDashboard = "dashboard"
)

// NewPRCmd creates the wtm pr command group.
func NewPRCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pr",
		Short:   "Manage pull requests",
		GroupID: domain.CmdGroupCore,
	}

	cmd.AddCommand(newPRListCmd())
	cmd.AddCommand(newPRCreateCmd())

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

	auth, err := ghservice.ResolveAuth()
	if errors.Is(err, domain.ErrAuthNotConfigured) || errors.Is(err, domain.ErrAuthNeedsReauth) {
		fmt.Fprintln(cmd.ErrOrStderr(), err.Error())
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

	if len(prs) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No open pull requests.")
		return nil
	}

	// Non-interactive mode (pipe)
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		output.PrintPRList(prs, cmd.OutOrStdout())
		return nil
	}

	// Interactive mode
	pr, action, err := pickPRAndAction(prs)
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil
		}
		return err
	}

	return executePRAction(cmd, action, pr, dir)
}

func pickPRAndAction(prs []domain.PRInfo) (domain.PRInfo, string, error) {
	prOptions := make([]huh.Option[int], 0, len(prs))
	for _, pr := range prs {
		label := fmt.Sprintf("#%-4d  %-40s  %s", pr.Number, truncate(pr.Title, 40), pr.Author)
		prOptions = append(prOptions, huh.NewOption(label, pr.Number))
	}

	var selected int
	var action string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[int]().
				Title("Select a pull request").
				Options(prOptions...).
				Value(&selected),
			huh.NewSelect[string]().
				Title("Action").
				Options(
					huh.NewOption("Open in browser", prActionBrowser),
					huh.NewOption("View details", prActionDetails),
					huh.NewOption("Open in dashboard", prActionDashboard),
				).
				Value(&action),
		),
	)

	if err := form.Run(); err != nil {
		return domain.PRInfo{}, "", err
	}

	for _, pr := range prs {
		if pr.Number == selected {
			return pr, action, nil
		}
	}

	return domain.PRInfo{}, "", fmt.Errorf("PR #%d not found", selected)
}

func executePRAction(cmd *cobra.Command, action string, pr domain.PRInfo, projectDir string) error {
	switch action {
	case prActionBrowser:
		return exec.Command("open", pr.URL).Run()

	case prActionDetails:
		fmt.Fprintln(cmd.OutOrStdout(), output.FormatPRDetailSection(pr))
		return nil

	case prActionDashboard:
		return runDashboardWithPR(cmd, projectDir, pr.Number)
	}

	return nil
}

func runDashboardWithPR(cmd *cobra.Command, projectDir string, prNumber int) error {
	result, ok := loadConfig(cmd, projectDir)
	if !ok {
		return nil
	}

	return launchDashboard(cmd, result, launchDashboardParams{InitialPR: &prNumber})
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

// projectRootFromCwd resolves the project root from the current directory.
func projectRootFromCwd() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	return projectRoot(dir)
}
