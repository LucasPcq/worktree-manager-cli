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
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/state"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

// newWtListCmd creates the wtm wt list subcommand.
func newWtListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all worktrees",
		Long:  "List all git worktrees with their status, PR info, and running services.",
		RunE:  runLs,
	}
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

	currentState, _ := state.Load()

	// Load PRs (graceful degradation)
	prs := loadPRsGraceful(result.ProjectDir)

	// Load services (graceful degradation)
	services := loadServicesGraceful()

	// Non-interactive mode
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprint(cmd.OutOrStdout(), output.FormatWorktreeList(output.FormatWorktreeListParams{
			Statuses:     statuses,
			ActiveBranch: currentState.ActiveWorktree,
			PRInfos:      prs,
			Services:     services,
		}))
		return nil
	}

	// Interactive mode
	if len(statuses) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No worktrees found.")
		return nil
	}

	selected, action, err := pickWorktreeAndAction(statuses, currentState.ActiveWorktree, prs, services)
	if errors.Is(err, domain.ErrUserAborted) {
		return nil
	}
	if err != nil {
		return err
	}

	return executeWorktreeAction(cmd, action, selected, prs, result)
}

func loadPRsGraceful(projectDir string) []domain.PRInfo {
	auth, err := ghservice.ResolveAuth()
	if err != nil {
		return nil
	}
	prs, err := ghservice.ListPRs(ghservice.ListPRsParams{
		ProjectDir: projectDir,
		Filter:     domain.PRFilterAll,
		Username:   auth.User,
	})
	if err != nil {
		return nil
	}
	return prs
}

func loadServicesGraceful() []process.ServiceInfo {
	socketPath := process.SocketPath()
	if !process.IsDaemonRunning(socketPath) {
		return nil
	}
	client := process.NewClient(socketPath)
	resp, err := client.Send(process.Request{Action: process.ActionList})
	if err != nil {
		return nil
	}
	return resp.Services
}

const (
	lsActionGo           = "go"
	lsActionFocus        = "focus"
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
	services []process.ServiceInfo,
) (domain.WorktreeStatus, string, error) {
	wtOptions := make([]huh.Option[int], 0, len(statuses))
	for i, s := range statuses {
		label := buildWorktreeLabel(s, activeBranch, prs, services)
		wtOptions = append(wtOptions, huh.NewOption(label, i))
	}

	var selectedIdx int
	var action string

	// Build action options dynamically after worktree selection
	// For now, use a static set — context-specific options will be shown
	actionOptions := []huh.Option[string]{
		huh.NewOption("Go (cd to worktree)", lsActionGo),
		huh.NewOption("Focus (swap env)", lsActionFocus),
		huh.NewOption("Start profile", lsActionServicesUp),
		huh.NewOption("Stop profile", lsActionServicesDown),
		huh.NewOption("View logs", lsActionLogs),
		huh.NewOption("Clean (delete worktree)", lsActionClean),
		huh.NewOption("Open in dashboard", lsActionDashboard),
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[int]().
				Title("Select a worktree").
				Options(wtOptions...).
				Value(&selectedIdx),
			huh.NewSelect[string]().
				Title("Action").
				Options(actionOptions...).
				Value(&action),
		),
	)

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return domain.WorktreeStatus{}, "", domain.ErrUserAborted
		}
		return domain.WorktreeStatus{}, "", err
	}

	return statuses[selectedIdx], action, nil
}

func buildWorktreeLabel(s domain.WorktreeStatus, activeBranch string, prs []domain.PRInfo, services []process.ServiceInfo) string {
	label := s.Branch

	var tags []string
	if s.IsParent {
		tags = append(tags, "parent")
	}
	if s.Branch == activeBranch {
		tags = append(tags, "active")
	}
	for _, pr := range prs {
		if pr.Branch == s.Branch {
			tags = append(tags, fmt.Sprintf("PR #%d", pr.Number))
			break
		}
	}
	for _, svc := range services {
		if svc.WorkDir == s.Path && svc.Status == domain.ServiceStatusRunning {
			tags = append(tags, "services")
			break
		}
	}
	if s.IsDirty {
		tags = append(tags, "dirty")
	}

	if len(tags) > 0 {
		label += "  (" + joinTags(tags) + ")"
	}

	return label
}

func joinTags(tags []string) string {
	result := ""
	for i, t := range tags {
		if i > 0 {
			result += ", "
		}
		result += t
	}
	return result
}

func executeWorktreeAction(cmd *cobra.Command, action string, selected domain.WorktreeStatus, prs []domain.PRInfo, result configResult) error {
	bin, err := os.Executable()
	if err != nil {
		return err
	}

	switch action {
	case lsActionGo:
		cmd := exec.Command(bin, "wt", "go", selected.Branch)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	case lsActionFocus:
		cmd := exec.Command(bin, "wt", "focus", selected.Branch)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	case lsActionServicesUp:
		cmd := exec.Command(bin, "svc", "up")
		cmd.Dir = selected.Path
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	case lsActionServicesDown:
		cmd := exec.Command(bin, "svc", "down")
		cmd.Dir = selected.Path
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	case lsActionLogs:
		cmd := exec.Command(bin, "svc", "logs")
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
