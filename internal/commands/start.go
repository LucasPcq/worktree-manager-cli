package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/service/process"
)

// newSvcStartCmd creates the wtm svc start subcommand.
func newSvcStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start <service>",
		Short: "Start a single service",
		Long:  "Start an individual service by name (defined in .wtm/services.toml).",
		Args:  cobra.ExactArgs(1),
		RunE:  runStart,
	}
	cmd.Flags().String(domain.FlagOutput, domain.OutputText, "Output format: text or json")
	return cmd
}

func runStart(cmd *cobra.Command, args []string) error {
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

	svc, ok := svcCfg.FindService(args[0])
	if !ok {
		return fmt.Errorf("service %q not found in config", args[0])
	}

	socketPath := process.SocketPath()
	if err := process.EnsureDaemon(socketPath); err != nil {
		return fmt.Errorf("ensure daemon: %w", err)
	}

	client := process.NewClient(socketPath)
	resp, err := client.Send(process.Request{
		Action:  process.ActionStart,
		Service: &svc,
		WorkDir: dir,
	})
	if err != nil {
		return fmt.Errorf("start %s: %w", svc.Name, err)
	}
	if resp.Status == process.StatusError {
		return fmt.Errorf("start %s: %s", svc.Name, resp.Message)
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	if format == domain.OutputJSON {
		return output.WriteServiceResultJSON(cmd.OutOrStdout(), output.ServiceActionResult{
			Name:   svc.Name,
			Status: domain.ServiceActionStarted,
		})
	}

	output.Blank(cmd.OutOrStdout())
	output.Success(cmd.OutOrStdout(), fmt.Sprintf("%s started", svc.Name))
	output.Blank(cmd.OutOrStdout())
	return nil
}
