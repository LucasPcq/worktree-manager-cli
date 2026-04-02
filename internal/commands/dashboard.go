package commands

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/tui/dashboard"
)

// NewDashboardCmd creates the wtm dashboard command (also the default).
func NewDashboardCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "dashboard",
		Short:  "Open the interactive dashboard",
		Hidden: true,
		RunE:   RunDashboard,
	}
}

// RunDashboard launches the interactive TUI dashboard.
func RunDashboard(cmd *cobra.Command, _ []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, ok := loadConfig(cmd, dir)
	if !ok {
		return nil
	}

	model := dashboard.New(dashboard.NewParams{
		Config:     result.Config,
		ProjectDir: result.ProjectDir,
	})

	program := tea.NewProgram(model, tea.WithAltScreen())
	_, err = program.Run()
	return err
}
