package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	svcpicker "github.com/LucasPcq/wtm/internal/tui/svc"
)

// newSvcListCmd creates the wtm svc list subcommand.
func newSvcListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List services and profiles declared in .wtm/services.toml",
		Long:  "Show the services and profiles configured for the project.\nIn a TTY, offers an interactive picker with start/stop/logs actions.",
		RunE:  runSvcList,
	}
	cmd.Flags().String(domain.FlagOutput, domain.OutputText, "Output format: text or json")
	return cmd
}

func runSvcList(cmd *cobra.Command, _ []string) error {
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

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	if format == domain.OutputJSON {
		return output.WriteServicesConfigJSON(cmd.OutOrStdout(), svcCfg)
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprint(cmd.OutOrStdout(), output.FormatServicesConfig(svcCfg))
		return nil
	}

	if len(svcCfg.Services) == 0 && len(svcCfg.Profiles) == 0 {
		output.Message(cmd.OutOrStdout(), "No services or profiles defined in .wtm/services.toml.")
		return nil
	}

	pick, err := svcpicker.RunListPicker(svcCfg)
	if errors.Is(err, domain.ErrUserAborted) {
		return nil
	}
	if err != nil {
		return err
	}

	return execSvcListAction(cmd, pick)
}

func execSvcListAction(cmd *cobra.Command, pick svcpicker.ListPickerResult) error {
	bin, err := os.Executable()
	if err != nil {
		return err
	}

	var args []string
	switch {
	case pick.Kind == svcpicker.KindProfile && pick.Action == svcpicker.ActionUp:
		args = []string{"svc", "up", pick.Name}
	case pick.Kind == svcpicker.KindProfile && pick.Action == svcpicker.ActionDown:
		args = []string{"svc", "down", pick.Name}
	case pick.Kind == svcpicker.KindService && pick.Action == svcpicker.ActionStart:
		args = []string{"svc", "start", pick.Name}
	case pick.Kind == svcpicker.KindService && pick.Action == svcpicker.ActionStop:
		args = []string{"svc", "stop", pick.Name}
	case pick.Kind == svcpicker.KindService && pick.Action == svcpicker.ActionLogs:
		args = []string{"svc", "logs", pick.Name}
	default:
		return nil
	}

	c := exec.Command(bin, args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
