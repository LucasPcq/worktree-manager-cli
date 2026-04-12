package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// newSvcUpCmd creates the wtm svc up subcommand.
func newSvcUpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up [profile]",
		Short: "Start a service profile",
		Long:  "Start all services in a profile.\nWithout arguments, starts the default profile (or shows a picker if multiple exist).",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runUp,
	}
}

func runUp(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	if _, ok := loadConfig(cmd, dir); !ok {
		return nil
	}

	svcCfg, err := config.LoadServices(dir)
	if err != nil {
		return fmt.Errorf("load services config: %w", err)
	}

	output.Blank(cmd.ErrOrStderr())
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

	for i := range services {
		svc := services[i]
		resp, sendErr := client.Send(process.Request{
			Action:  process.ActionStart,
			Service: &svc,
			WorkDir: dir,
		})
		if sendErr != nil {
			output.Error(cmd.ErrOrStderr(), fmt.Sprintf("%s: %v", svc.Name, sendErr))
			continue
		}
		if resp.Status == process.StatusError {
			output.Error(cmd.ErrOrStderr(), fmt.Sprintf("%s: %s", svc.Name, resp.Message))
			continue
		}
		output.Success(cmd.OutOrStdout(), fmt.Sprintf("%s started", svc.Name))
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

	selected, err := components.RunStandaloneSelect(sl)
	if err != nil {
		if errors.Is(err, components.ErrAborted) {
			return domain.ProfileConfig{}, domain.ErrUserAborted
		}
		return domain.ProfileConfig{}, err
	}

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
