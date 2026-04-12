package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// newSvcUpCmd creates the wtm svc up subcommand.
func newSvcUpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up [profile]",
		Short: "Start a service profile",
		Long:  "Start all services in a profile.\nWithout arguments, starts the default profile (or shows a picker if multiple exist).",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runUp,
	}

	cmd.Flags().Bool(domain.FlagExclusive, false, "Stop services on other worktrees before starting")
	cmd.Flags().Bool(domain.FlagParallel, false, "Start without stopping other worktrees")
	cmd.Flags().String(domain.FlagOutput, domain.OutputText, "Output format: text or json")

	return cmd
}

func runUp(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, ok := loadConfig(cmd, dir)
	if !ok {
		return nil
	}

	svcCfg, err := config.LoadServices(result.ProjectDir)
	if err != nil {
		return fmt.Errorf("load services config: %w", err)
	}

	for _, warning := range config.ValidateServices(svcCfg) {
		output.Warning(cmd.ErrOrStderr(), warning)
	}

	services, err := resolveProfileServices(args, svcCfg)
	if err != nil {
		return err
	}

	socketPath := process.SocketPath()
	if err := process.EnsureDaemon(socketPath); err != nil {
		return fmt.Errorf("ensure daemon: %w", err)
	}

	client := process.NewClient(socketPath)

	if err := handleConcurrentServices(cmd, client, dir); err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	results := make([]output.ServiceActionResult, 0, len(services))

	for i := range services {
		svc := services[i]
		stopSpinner := startSpinner(cmd.ErrOrStderr(), fmt.Sprintf("Starting %s...", svc.Name))
		resp, sendErr := client.Send(process.Request{
			Action:  process.ActionStart,
			Service: &svc,
			WorkDir: dir,
		})
		stopSpinner()
		if sendErr != nil {
			results = append(results, output.ServiceActionResult{Name: svc.Name, Status: domain.ServiceActionError, Message: sendErr.Error()})
			if format != domain.OutputJSON {
				output.Error(cmd.ErrOrStderr(), fmt.Sprintf("%s: %v", svc.Name, sendErr))
			}
			continue
		}
		if resp.Status == process.StatusError {
			results = append(results, output.ServiceActionResult{Name: svc.Name, Status: domain.ServiceActionError, Message: resp.Message})
			if format != domain.OutputJSON {
				output.Error(cmd.ErrOrStderr(), resp.Message)
			}
			continue
		}
		results = append(results, output.ServiceActionResult{Name: svc.Name, Status: domain.ServiceActionStarted})
		if format != domain.OutputJSON {
			output.Success(cmd.OutOrStdout(), fmt.Sprintf("%s started", svc.Name))
		}
	}

	if format == domain.OutputJSON {
		return output.WriteServiceResultsJSON(cmd.OutOrStdout(), results)
	}

	output.Blank(cmd.OutOrStdout())
	return nil
}

func resolveProfileServices(args []string, svcCfg domain.ServicesConfig) ([]domain.ServiceConfig, error) {
	if len(args) > 0 {
		profile, ok := svcCfg.FindProfile(args[0])
		if !ok {
			return nil, fmt.Errorf("profile %q not found in config", args[0])
		}
		return svcCfg.ProfileServices(profile), nil
	}

	// 1 profile or less → use default
	if len(svcCfg.Profiles) <= 1 {
		profile, ok := svcCfg.DefaultProfile()
		if !ok {
			return svcCfg.Services, nil
		}
		return svcCfg.ProfileServices(profile), nil
	}

	// 2+ profiles → interactive picker
	profile, err := pickProfile(svcCfg)
	if err != nil {
		return nil, err
	}

	return svcCfg.ProfileServices(profile), nil
}

func pickProfile(svcCfg domain.ServicesConfig) (domain.ProfileConfig, error) {
	defaultProfile, _ := svcCfg.DefaultProfile()

	items := make([]components.SelectItem, 0, len(svcCfg.Profiles))
	for _, p := range svcCfg.Profiles {
		label := p.Name
		if len(p.Services) > 0 {
			label += fmt.Sprintf(" (%s)", joinServiceNames(p.Services))
		}
		items = append(items, components.SelectItem{Label: label, Value: p.Name})
	}

	sl := components.NewSelectList(components.NewSelectListParams{
		Title:       "Select profile",
		Description: "Which services to start?",
		Items:       items,
	})

	selected := defaultProfile.Name
	result, err := components.RunStandaloneSelect(sl)
	if err != nil {
		return domain.ProfileConfig{}, domain.ErrUserAborted
	}
	selected = result

	profile, ok := svcCfg.FindProfile(selected)
	if !ok {
		return domain.ProfileConfig{}, fmt.Errorf("profile %q not found", selected)
	}

	return profile, nil
}

func joinServiceNames(names []string) string {
	result := ""
	for i, n := range names {
		if i > 0 {
			result += ", "
		}
		result += n
	}
	return result
}

// handleConcurrentServices checks if services are running on other worktrees
// and handles the exclusive/parallel decision.
func handleConcurrentServices(cmd *cobra.Command, client *process.Client, currentDir string) error {
	exclusiveFlag, _ := cmd.Flags().GetBool(domain.FlagExclusive)
	parallelFlag, _ := cmd.Flags().GetBool(domain.FlagParallel)

	if parallelFlag {
		return nil
	}

	otherWorktrees, otherNames := findOtherRunningServices(client, currentDir)
	if len(otherWorktrees) == 0 {
		return nil
	}

	if exclusiveFlag {
		return stopOtherServices(client, otherWorktrees, cmd)
	}

	return promptConcurrentServices(cmd, client, otherWorktrees, otherNames)
}

func findOtherRunningServices(client *process.Client, currentDir string) (map[string]bool, map[string][]string) {
	resp, err := client.Send(process.Request{Action: process.ActionList})
	if err != nil {
		return nil, nil
	}

	otherWorktrees := make(map[string]bool)
	otherNames := make(map[string][]string)

	for _, svc := range resp.Services {
		if svc.Status != domain.ServiceStatusRunning {
			continue
		}
		if svc.WorkDir == currentDir {
			continue
		}
		otherWorktrees[svc.WorkDir] = true
		otherNames[svc.WorkDir] = append(otherNames[svc.WorkDir], svc.Name)
	}

	return otherWorktrees, otherNames
}

func promptConcurrentServices(cmd *cobra.Command, client *process.Client, otherWorktrees map[string]bool, otherNames map[string][]string) error {
	output.Blank(cmd.ErrOrStderr())
	for dir, names := range otherNames {
		short := filepath.Base(dir)
		output.Warning(cmd.ErrOrStderr(), fmt.Sprintf("Services running on %s (%s)", short, strings.Join(names, ", ")))
	}

	items := []components.SelectItem{
		{Label: "Yes, stop and start here", Value: "yes"},
		{Label: "No, run in parallel", Value: "no"},
	}

	sl := components.NewSelectList(components.NewSelectListParams{
		Title: "Stop other services before starting?",
		Items: items,
	})

	choice, err := components.RunStandaloneSelect(sl)
	if err != nil {
		return domain.ErrUserAborted
	}

	if choice == "yes" {
		output.Blank(cmd.ErrOrStderr())
		return stopOtherServices(client, otherWorktrees, cmd)
	}
	return nil
}

func stopOtherServices(client *process.Client, worktrees map[string]bool, cmd *cobra.Command) error {
	for dir := range worktrees {
		resp, err := client.Send(process.Request{
			Action:  process.ActionStopAll,
			WorkDir: dir,
		})
		if err != nil {
			output.Error(cmd.ErrOrStderr(), fmt.Sprintf("stop services in %s: %v", filepath.Base(dir), err))
			continue
		}
		if resp.Status == process.StatusError {
			output.Error(cmd.ErrOrStderr(), fmt.Sprintf("stop services in %s: %s", filepath.Base(dir), resp.Message))
			continue
		}
		output.Success(cmd.ErrOrStderr(), fmt.Sprintf("Stopped services in %s", filepath.Base(dir)))
	}
	return nil
}

