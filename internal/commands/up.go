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

// NewUpCmd creates the wtm up command.
func NewUpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up [service]",
		Short: "Start services defined in .wtm.services.toml",
		Long:  "Start a single service by name, or all services in a profile.\nWithout arguments, starts the default profile.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runUp,
	}

	cmd.Flags().String(domain.FlagProfile, "", "Profile to start (defaults to the default profile)")

	return cmd
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

	services, err := resolveServices(cmd, args, svcCfg)
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

func resolveServices(cmd *cobra.Command, args []string, svcCfg domain.ServicesConfig) ([]domain.ServiceConfig, error) {
	if len(args) > 0 {
		svc, ok := svcCfg.FindService(args[0])
		if !ok {
			return nil, fmt.Errorf("service %q not found in config", args[0])
		}
		return []domain.ServiceConfig{svc}, nil
	}

	profileName, _ := cmd.Flags().GetString(domain.FlagProfile)

	if profileName != "" {
		profile, ok := svcCfg.FindProfile(profileName)
		if !ok {
			return nil, fmt.Errorf("profile %q not found in config", profileName)
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
