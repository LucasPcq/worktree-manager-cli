package wt

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/tui/components"
	"github.com/LucasPcq/wtm/internal/tui/worktreepicker"
)

// newListCmd creates the wtm wt list subcommand.
func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdList,
		Short: "List all worktrees",
		Long:  "List all git worktrees with their status, PR info, and running services.",
		RunE:  runList,
	}
	shared.AddOutputFlag(cmd)
	return cmd
}

func runList(cmd *cobra.Command, _ []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, ok := shared.LoadConfig(cmd, dir)
	if !ok {
		return nil
	}

	statuses, err := worktree.List(domain.ListParams{
		ProjectDir: result.ProjectDir,
		StateDir:   result.StateDir,
		Config:     result.Config,
	})
	if err != nil {
		return fmt.Errorf("list worktrees: %w", err)
	}

	prs := shared.LoadPRsGraceful(result.ProjectDir)
	services := shared.LoadJobsGraceful()

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	if format == domain.OutputJSON {
		return output.WriteWorktreeListJSON(cmd.OutOrStdout(), output.WriteWorktreeListJSONParams{
			Statuses: statuses,
			PRInfos:  prs,
			Services: services,
		})
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprint(cmd.OutOrStdout(), output.FormatWorktreeList(output.FormatWorktreeListParams{
			Statuses:     statuses,
			ActiveBranch: "",
			PRInfos:      prs,
			Services:     services,
		}))
		return nil
	}

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

const (
	lsActionGo           = "go"
	lsActionSwitch       = "switch"
	lsActionServicesUp   = "services-up"
	lsActionServicesDown = "services-down"
	lsActionLogs         = "logs"
	lsActionClean        = "clean"
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

func executeWorktreeAction(cmd *cobra.Command, action string, selected domain.WorktreeStatus, prs []domain.PRInfo, result shared.ConfigResult) error {
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
		c := exec.Command(bin, domain.CmdRun, domain.CmdUp)
		c.Dir = selected.Path
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()

	case lsActionServicesUp:
		cmd := exec.Command(bin, domain.CmdRun, domain.CmdUp)
		cmd.Dir = selected.Path
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	case lsActionServicesDown:
		cmd := exec.Command(bin, domain.CmdRun, domain.CmdDown)
		cmd.Dir = selected.Path
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	case lsActionLogs:
		cmd := exec.Command(bin, domain.CmdRun, domain.CmdLogs)
		cmd.Dir = selected.Path
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	case lsActionClean:
		cmd := exec.Command(bin, domain.CmdWt, domain.CmdClean, selected.Branch)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	return nil
}
