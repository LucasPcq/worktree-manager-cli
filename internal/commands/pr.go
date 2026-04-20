package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/output"
	ghservice "github.com/LucasPcq/wtm/internal/service/github"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

const (
	prActionBrowser  = "browser"
	prActionDetails  = "details"
	prActionCheckout = "checkout"
	prActionGo       = "go"
)

// NewPRCmd creates the wtm pr command group.
func NewPRCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     domain.CmdPr,
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
		Use:   domain.CmdList,
		Short: "List open pull requests",
		RunE:  runPRList,
	}

	cmd.Flags().Bool(domain.FlagReview, false, "Show only PRs where you are requested as reviewer")
	cmd.Flags().Bool(domain.FlagMine, false, "Show only your PRs")
	addOutputFlag(cmd)

	return cmd
}

func runPRList(cmd *cobra.Command, _ []string) error {
	dir, err := projectRootFromCwd()
	if err != nil {
		return err
	}

	filter := domain.PRFilterAll

	review, _ := cmd.Flags().GetBool(domain.FlagReview)
	mine, _ := cmd.Flags().GetBool(domain.FlagMine)

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

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	if format == domain.OutputJSON {
		return output.WritePRListJSON(cmd.OutOrStdout(), prs)
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

	// Collect existing worktree branches for contextual actions
	existingBranches := worktreeBranches(dir)

	// Interactive mode
	pr, action, err := pickPRAndAction(prs, existingBranches)
	if err != nil {
		if errors.Is(err, domain.ErrUserAborted) {
			return nil
		}
		return err
	}

	return executePRAction(cmd, action, pr, dir)
}

func pickPRAndAction(prs []domain.PRInfo, existingBranches []string) (domain.PRInfo, string, error) {
	// Step 1: pick a PR
	prItems := make([]components.SelectItem, 0, len(prs))
	for _, pr := range prs {
		label := fmt.Sprintf("#%-4d  %-40s  %s", pr.Number, truncate(pr.Title, 40), pr.Author)
		prItems = append(prItems, components.SelectItem{
			Label: label,
			Value: strconv.Itoa(pr.Number),
		})
	}

	selected, err := components.RunStandaloneSelect(
		components.NewSelectList(components.NewSelectListParams{Title: "Select a pull request", Items: prItems}),
	)
	if err != nil {
		if errors.Is(err, components.ErrAborted) {
			return domain.PRInfo{}, "", domain.ErrUserAborted
		}
		return domain.PRInfo{}, "", err
	}

	prNum, err := strconv.Atoi(selected)
	if err != nil {
		return domain.PRInfo{}, "", fmt.Errorf("parse PR number: %w", err)
	}

	var selectedPR domain.PRInfo
	for _, pr := range prs {
		if pr.Number == prNum {
			selectedPR = pr
			break
		}
	}
	if selectedPR.Number == 0 {
		return domain.PRInfo{}, "", fmt.Errorf("PR #%d not found", prNum)
	}

	// Step 2: build contextual action items based on worktree existence
	branchHasWorktree := containsString(existingBranches, selectedPR.Branch)

	var actionItems []components.SelectItem
	if branchHasWorktree {
		actionItems = append(actionItems, components.SelectItem{Label: "Go to worktree", Value: prActionGo})
	} else {
		actionItems = append(actionItems, components.SelectItem{Label: "Checkout into worktree", Value: prActionCheckout})
	}

	actionItems = append(actionItems, components.SelectItem{Separator: true})
	actionItems = append(actionItems, components.SelectItem{Label: "Open in browser", Value: prActionBrowser})
	actionItems = append(actionItems, components.SelectItem{Label: "View details", Value: prActionDetails})

	action, err := components.RunStandaloneSelect(
		components.NewSelectList(components.NewSelectListParams{
			Title: fmt.Sprintf("#%d — %s", selectedPR.Number, truncate(selectedPR.Title, 30)),
			Items: actionItems,
		}),
	)
	if err != nil {
		if errors.Is(err, components.ErrAborted) {
			return domain.PRInfo{}, "", domain.ErrUserAborted
		}
		return domain.PRInfo{}, "", err
	}

	return selectedPR, action, nil
}

func executePRAction(cmd *cobra.Command, action string, pr domain.PRInfo, projectDir string) error {
	switch action {
	case prActionBrowser:
		return exec.Command("open", pr.URL).Run()

	case prActionDetails:
		output.PrintPRDetail(cmd.OutOrStdout(), pr)
		return nil

	case prActionCheckout:
		result, ok := loadConfig(cmd, projectDir)
		if !ok {
			return nil
		}
		return checkoutPR(cmd, result, checkoutPRParams{Number: pr.Number})

	case prActionGo:
		result, err := worktree.Resolve(worktree.ResolveParams{
			ProjectDir: projectDir,
			Query:      pr.Branch,
		})
		if err != nil {
			return fmt.Errorf("resolve worktree: %w", err)
		}
		goFile := os.Getenv(domain.EnvGoFile)
		if goFile != "" {
			return os.WriteFile(goFile, []byte(result.Path), 0o644)
		}
		fmt.Println(result.Path)
		return nil
	}

	return nil
}

// worktreeBranches returns the branch names of all existing worktrees (graceful degradation).
func worktreeBranches(projectDir string) []string {
	wts, err := infra.ListWorktrees(infra.ListWorktreesParams{ProjectDir: projectDir})
	if err != nil {
		return nil
	}
	branches := make([]string, 0, len(wts))
	for _, wt := range wts {
		branches = append(branches, wt.Branch)
	}
	return branches
}

func containsString(ss []string, target string) bool {
	for _, s := range ss {
		if s == target {
			return true
		}
	}
	return false
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
