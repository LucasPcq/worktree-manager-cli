package shell

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/shell"
)

// NewCmd creates the wtm shell-init command.
func NewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "shell-init [zsh|bash|fish|powershell]",
		Short: "Generate shell integration function",
		Long: "Output a shell function to eval in your rc file.\n\n" +
			"The shell is detected from $SHELL; pass it explicitly when detection\n" +
			"cannot work — notably PowerShell, which does not set $SHELL.\n\n" +
			"Usage: " + domain.ShellInitCommand + "\n" +
			"PowerShell: " + domain.ShellInitCommandPowerShell,
		Args: cobra.MaximumNArgs(1),
		RunE: runShellInit,
	}
}

func runShellInit(cmd *cobra.Command, args []string) error {
	target, err := resolveShell(args)
	if err != nil {
		return err
	}

	fmt.Fprint(cmd.OutOrStdout(), rules.GenerateShellInit(target))
	return nil
}

func resolveShell(args []string) (domain.ShellType, error) {
	if len(args) == 0 {
		return shell.DetectShell(), nil
	}
	return rules.ParseShell(args[0])
}
