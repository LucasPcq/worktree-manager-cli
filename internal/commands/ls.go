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
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/tui/components"
	"github.com/LucasPcq/wtm/internal/tui/worktreepicker"
)

// newWtListCmd creates the wtm wt list subcommand.
func newWtListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all worktrees",
		Long:  "List all git worktrees with their status, PR info, and running services.",
		RunE:  runLs,
	}
	cmd.Flags().String(domain.FlagOutput, domain.OutputText, "Output format: text or json")
	return cmd
}

func runLs(cmd *cobra.Command, _ []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, ok := loadConfig(cmd, dir)
	if !ok {
		return nil
	}

	statuses, err := worktree.List(worktree.ListParams{
		ProjectDir: result.ProjectDir,
		Config:     result.Config,
	})
	if err != nil {
		return fmt.Errorf("list worktrees: %w", err)
	}

	// Load PRs (graceful degradation)
	prs := loadPRsGraceful(result.ProjectDir)

	// Load services (graceful degradation)
	services := loadJobsGraceful()

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	if format == domain.OutputJSON {
		return output.WriteWorktreeListJSON(cmd.OutOrStdout(), output.WriteWorktreeListJSONParams{
			Statuses: statuses,
			PRInfos:  prs,
			Services: services,
		})
	}

	// Non-interactive mode
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprint(cmd.OutOrStdout(), output.FormatWorktreeList(output.FormatWorktreeListParams{
			Statuses:     statuses,
			ActiveBranch: "",
			PRInfos:      prs,
			Services:     services,
		}))
		return nil
	}

	// Interactive mode
	if len(statuses) == 0 {
		output.Message(cmd.OutOrStdout(), "No worktrees found.")
		return nil
	}

	selected, action, err := pickWorktreeAndAction(statuses, "", prs, services)
	if errors.Is(err, domain.ErrUserAborted) {
		return nil
	}
	if err != nil {
		return err
	}

	return executeWorktreeAction(cmd, action, selected, prs, result)
}

func loadPRsGraceful(projectDir string) []domain.PRInfo {
	prs, err := ghservice.ListPRs(ghservice.ListPRsParams{
		ProjectDir: projectDir,
		Filter:     domain.PRFilterAll,
	})
	if err != nil {
		return nil
	}
	return prs
}

func loadJobsGraceful() []process.JobInfo {
	socketPath := process.SocketPath()
	if !process.IsDaemonRunning(socketPath) {
		return nil
	}
	client := process.NewClient(socketPath)
	resp, err := client.Send(process.Request{Action: process.ActionList})
	if err != nil {
		return nil
	}
	return resp.Jobs
}

const (
	lsActionGo           = "go"
	lsActionSwitch       = "switch"
	lsActionServicesUp   = "services-up"
	lsActionServicesDown = "services-down"
	lsActionLogs         = "logs"
	lsActionClean        = "clean"
	lsActionCreatePR     = "create-pr"
	lsActionOpenPR       = "open-pr"
	lsActionDashboard    = "dashboard"
)

func pickWorktreeAndAction(
	statuses []domain.WorktreeStatus,
	activeBranch string,
	prs []domain.PRInfo,
	services []process.JobInfo,
) (domain.WorktreeStatus, string, error) {
	wtItems := make([]components.SelectItem, 0, len(statuses))
	for i, s := range statuses {
		wtItems = append(wtItems, components.SelectItem{
			Label:  s.Branch,
			Value:  strconv.Itoa(i),
			Badges: worktreepicker.BuildBadges(s, prs, services),
		})
	}

	// Map index → branch name for the summary display
	branchByIdx := make(map[string]string, len(statuses))
	for i, s := range statuses {
		branchByIdx[strconv.Itoa(i)] = s.Branch
	}

	actionItems := []components.SelectItem{
		{Label: "Go (cd to worktree)", Value: lsActionGo},
		{Label: "Switch (go + start services)", Value: lsActionSwitch},
		{Separator: true},
		{Label: "Start profile", Value: lsActionServicesUp},
		{Label: "Stop profile", Value: lsActionServicesDown},
		{Label: "View logs", Value: lsActionLogs},
		{Separator: true},
		{Label: "Clean (delete worktree)", Value: lsActionClean, Danger: true},
	}
	if domain.FeatureDashboard {
		actionItems = append(actionItems, components.SelectItem{Label: "Open in dashboard", Value: lsActionDashboard})
	}

	wiz := components.NewWizard([]components.Step{
		{
			Name:  "Worktree",
			Model: components.NewSelectList(components.NewSelectListParams{Title: "Select a worktree", Items: wtItems}),
			Summary: func(m any) string {
				sl, ok := m.(components.SelectListModel)
				if !ok {
					return ""
				}
				if name, found := branchByIdx[sl.Value()]; found {
					return name
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
		return domain.WorktreeStatus{}, "", fmt.Errorf("wizard: %w", err)
	}

	final, ok := finalModel.(components.WizardModel)
	if !ok || final.Aborted() {
		return domain.WorktreeStatus{}, "", domain.ErrUserAborted
	}

	wtSL, ok := final.Steps()[0].Model.(components.SelectListModel)
	if !ok {
		return domain.WorktreeStatus{}, "", fmt.Errorf("unexpected model type for worktree step")
	}

	idx, err := strconv.Atoi(wtSL.Value())
	if err != nil {
		return domain.WorktreeStatus{}, "", fmt.Errorf("parse worktree index: %w", err)
	}

	actionSL, ok := final.Steps()[1].Model.(components.SelectListModel)
	if !ok {
		return domain.WorktreeStatus{}, "", fmt.Errorf("unexpected model type for action step")
	}

	return statuses[idx], actionSL.Value(), nil
}

func executeWorktreeAction(cmd *cobra.Command, action string, selected domain.WorktreeStatus, prs []domain.PRInfo, result configResult) error {
	bin, err := os.Executable()
	if err != nil {
		return err
	}

	switch action {
	case lsActionGo:
		goFile := os.Getenv(domain.EnvGoFile)
		if goFile != "" {
			return os.WriteFile(goFile, []byte(selected.Path), 0o644)
		}
		fmt.Println(selected.Path)
		return nil

	case lsActionSwitch:
		goFile := os.Getenv(domain.EnvGoFile)
		if goFile != "" {
			if err := os.WriteFile(goFile, []byte(selected.Path), 0o644); err != nil {
				return err
			}
		}
		c := exec.Command(bin, "run", "up")
		c.Dir = selected.Path
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()

	case lsActionServicesUp:
		cmd := exec.Command(bin, "run", "up")
		cmd.Dir = selected.Path
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	case lsActionServicesDown:
		cmd := exec.Command(bin, "run", "down")
		cmd.Dir = selected.Path
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	case lsActionLogs:
		cmd := exec.Command(bin, "run", "logs")
		cmd.Dir = selected.Path
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	case lsActionClean:
		cmd := exec.Command(bin, "wt", "clean", selected.Branch)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	case lsActionCreatePR:
		cmd := exec.Command(bin, "pr", "create")
		cmd.Dir = selected.Path
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	case lsActionOpenPR:
		for _, pr := range prs {
			if pr.Branch == selected.Branch {
				return exec.Command("open", pr.URL).Run()
			}
		}

	case lsActionDashboard:
		branch := selected.Branch
		return launchDashboard(cmd, result, launchDashboardParams{InitialBranch: &branch})
	}

	return nil
}
