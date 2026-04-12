package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	ghservice "github.com/LucasPcq/wtm/internal/service/github"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

const (
	prActionBrowser   = "browser"
	prActionDetails   = "details"
	prActionDashboard = "dashboard"
	prActionCheckout  = "checkout"
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
	cmd.AddCommand(newPRCheckoutCmd())

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
	})
	stop()
	if err != nil {
		return fmt.Errorf("list PRs: %w", err)
	}

	if len(prs) == 0 {
		output.Message(cmd.OutOrStdout(), "No open pull requests.")
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
		if errors.Is(err, domain.ErrUserAborted) {
			return nil
		}
		return err
	}

	return executePRAction(cmd, action, pr, dir)
}

func pickPRAndAction(prs []domain.PRInfo) (domain.PRInfo, string, error) {
	prItems := make([]components.SelectItem, 0, len(prs))
	for _, pr := range prs {
		label := fmt.Sprintf("#%-4d  %-40s  %s", pr.Number, truncate(pr.Title, 40), pr.Author)
		prItems = append(prItems, components.SelectItem{
			Label: label,
			Value: strconv.Itoa(pr.Number),
		})
	}

	actionItems := []components.SelectItem{
		{Label: "Checkout into worktree", Value: prActionCheckout},
		{Label: "Open in browser", Value: prActionBrowser},
		{Label: "View details", Value: prActionDetails},
	}
	if domain.FeatureDashboard {
		actionItems = append(actionItems, components.SelectItem{Label: "Open in dashboard", Value: prActionDashboard})
	}

	wiz := components.NewWizard([]components.Step{
		{
			Name:  "Pull request",
			Model: components.NewSelectList(components.NewSelectListParams{Title: "Select a pull request", Items: prItems}),
			Summary: func(m any) string {
				sl, ok := m.(components.SelectListModel)
				if !ok {
					return ""
				}
				return sl.Value()
			},
		},
		{
			Name:  "Action",
			Model: components.NewSelectList(components.NewSelectListParams{Title: "Action", Items: actionItems}),
			Summary: func(m any) string {
				sl, ok := m.(components.SelectListModel)
				if !ok {
					return ""
				}
				return sl.Value()
			},
		},
	})

	finalModel, err := tea.NewProgram(wiz).Run()
	if err != nil {
		return domain.PRInfo{}, "", fmt.Errorf("wizard: %w", err)
	}

	final, ok := finalModel.(components.WizardModel)
	if !ok || final.Aborted() {
		return domain.PRInfo{}, "", domain.ErrUserAborted
	}

	prSL, ok := final.Steps()[0].Model.(components.SelectListModel)
	if !ok {
		return domain.PRInfo{}, "", fmt.Errorf("unexpected model type for PR step")
	}

	prNum, err := strconv.Atoi(prSL.Value())
	if err != nil {
		return domain.PRInfo{}, "", fmt.Errorf("parse PR number: %w", err)
	}

	actionSL, ok := final.Steps()[1].Model.(components.SelectListModel)
	if !ok {
		return domain.PRInfo{}, "", fmt.Errorf("unexpected model type for action step")
	}

	for _, pr := range prs {
		if pr.Number == prNum {
			return pr, actionSL.Value(), nil
		}
	}

	return domain.PRInfo{}, "", fmt.Errorf("PR #%d not found", prNum)
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

	case prActionCheckout:
		result, ok := loadConfig(cmd, projectDir)
		if !ok {
			return nil
		}
		return checkoutPR(cmd, result, checkoutPRParams{Number: pr.Number})
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

// pickPRNumber fetches the list of open PRs and shows a picker.
// Returns the selected PR number, or 0 if the user aborts.
func pickPRNumber(cmd *cobra.Command, projectDir string) (int, error) {
	stop := startSpinner(cmd.ErrOrStderr(), "Fetching pull requests...")
	prs, err := ghservice.ListPRs(ghservice.ListPRsParams{
		ProjectDir: projectDir,
		Filter:     domain.PRFilterAll,
	})
	stop()
	if err != nil {
		return 0, fmt.Errorf("list PRs: %w", err)
	}

	if len(prs) == 0 {
		output.Message(cmd.OutOrStdout(), "No open pull requests.")
		return 0, nil
	}

	prItems := make([]components.SelectItem, 0, len(prs))
	for _, pr := range prs {
		label := fmt.Sprintf("#%-4d  %-40s  %s", pr.Number, truncate(pr.Title, 40), pr.Author)
		prItems = append(prItems, components.SelectItem{
			Label: label,
			Value: strconv.Itoa(pr.Number),
		})
	}

	sl := components.NewSelectList(components.NewSelectListParams{
		Title: "Select a pull request to checkout",
		Items: prItems,
	})

	selected, err := components.RunStandaloneSelect(sl)
	if err != nil {
		if errors.Is(err, components.ErrAborted) {
			return 0, nil
		}
		return 0, err
	}

	num, err := strconv.Atoi(selected)
	if err != nil {
		return 0, fmt.Errorf("parse PR number: %w", err)
	}

	return num, nil
}
