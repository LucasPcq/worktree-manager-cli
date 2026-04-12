package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	svcpicker "github.com/LucasPcq/wtm/internal/tui/svc"
)

// newSvcPsCmd creates the wtm svc ps subcommand.
func newSvcPsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ps",
		Short: "List currently running services",
		Long:  "Show the services managed by the background daemon (name, status, PID, worktree).\nIn a TTY, offers an interactive picker with stop/logs/restart actions.",
		RunE:  runSvcPs,
	}
	cmd.Flags().String(domain.FlagOutput, domain.OutputText, "Output format: text or json")
	return cmd
}

func runSvcPs(cmd *cobra.Command, _ []string) error {
	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	services := loadServicesGraceful()

	if format == domain.OutputJSON {
		return output.WriteRunningServicesJSON(cmd.OutOrStdout(), services)
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprint(cmd.OutOrStdout(), output.FormatRunningServices(services))
		return nil
	}

	if len(services) == 0 {
		output.Message(cmd.OutOrStdout(), "No services running.")
		return nil
	}

	pick, err := svcpicker.RunPsPicker(services)
	if errors.Is(err, domain.ErrUserAborted) {
		return nil
	}
	if err != nil {
		return err
	}

	return execSvcPsAction(cmd, pick)
}

func execSvcPsAction(cmd *cobra.Command, pick svcpicker.PsPickerResult) error {
	bin, err := os.Executable()
	if err != nil {
		return err
	}

	var args []string
	switch pick.Action {
	case svcpicker.ActionPsStop:
		args = []string{"svc", "stop", pick.Name}
	case svcpicker.ActionPsLogs:
		args = []string{"svc", "logs", pick.Name}
	case svcpicker.ActionPsRestart:
		args = []string{"svc", "start", pick.Name}
	case svcpicker.ActionPsStopAll:
		args = []string{"svc", "down", "--all"}
	default:
		return nil
	}

	c := exec.Command(bin, args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
