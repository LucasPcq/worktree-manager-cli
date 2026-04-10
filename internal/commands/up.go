package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
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

	for _, warning := range config.ValidateServices(svcCfg) {
		fmt.Fprintf(cmd.ErrOrStderr(), "⚠ %s\n", warning)
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
			fmt.Fprintf(cmd.ErrOrStderr(), "  ✗ %s: %v\n", svc.Name, sendErr)
			continue
		}
		if resp.Status == process.StatusError {
			fmt.Fprintf(cmd.ErrOrStderr(), "  ✗ %s: %s\n", svc.Name, resp.Message)
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s started\n", svc.Name)
	}

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

	options := make([]huh.Option[string], 0, len(svcCfg.Profiles))
	for _, p := range svcCfg.Profiles {
		label := p.Name
		if len(p.Services) > 0 {
			label += fmt.Sprintf(" (%s)", joinServiceNames(p.Services))
		}
		options = append(options, huh.NewOption(label, p.Name))
	}

	selected := defaultProfile.Name

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select profile").
				Description("Which services to start?").
				Options(options...).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
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
