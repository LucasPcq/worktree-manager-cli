package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
)

// newRunExportCmd creates the wtm run export subcommand.
func newRunExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdExport,
		Short: "Export .wtm/run.toml as JSON on stdout",
		Long:  "Emit the current run config as JSON. Pipe to a file and use with wtm run import to share configurations.",
		RunE:  runRunExport,
	}
	cmd.Flags().String(domain.FlagProfile, "", "Export only this profile and its jobs")
	return cmd
}

func runRunExport(cmd *cobra.Command, _ []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, ok := loadConfig(cmd, dir)
	if !ok {
		return nil
	}

	runCfg, err := config.LoadRun(result.ProjectDir)
	if err != nil {
		return fmt.Errorf("load run config: %w", err)
	}

	profile, _ := cmd.Flags().GetString(domain.FlagProfile)
	if profile != "" {
		runCfg, err = runCfg.FilterToProfile(profile)
		if err != nil {
			return err
		}
	}

	return output.WriteRunConfigJSON(cmd.OutOrStdout(), runCfg)
}
