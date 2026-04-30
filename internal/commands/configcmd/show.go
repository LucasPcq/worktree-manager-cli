package configcmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
)

func newShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print the project config.toml",
		RunE:  runShow,
	}
}

func runShow(cmd *cobra.Command, _ []string) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	stateDir, err := shared.StateDir(wd)
	if err != nil {
		return err
	}

	path := filepath.Join(stateDir, domain.ConfigFileName)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		output.Warning(cmd.ErrOrStderr(), fmt.Sprintf("No config at %s. Run `wtm init` first.", path))
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	output.Blank(cmd.OutOrStdout())
	output.InfoLine(cmd.OutOrStdout(), "path", path)
	output.Blank(cmd.OutOrStdout())
	if _, err := cmd.OutOrStdout().Write(data); err != nil {
		return err
	}
	output.Blank(cmd.OutOrStdout())
	return nil
}
