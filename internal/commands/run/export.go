package run

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
)

// newExportCmd creates the wtm run export subcommand.
func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdExport,
		Short: "Export run.toml as JSON on stdout",
		Long:  "Emit the current run config as JSON. Pipe to a file and use with wtm run import to share configurations.",
		RunE:  runExport,
	}
	cmd.Flags().String(domain.FlagProfile, "", "Export only this profile and its jobs")
	return cmd
}

func runExport(cmd *cobra.Command, _ []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	runCfg, err := config.LoadRun(result.StateDir)
	if err != nil {
		return fmt.Errorf("load run config: %w", err)
	}

	if err := shared.RequireRunInitialized(runCfg); err != nil {
		return err
	}

	profile, _ := cmd.Flags().GetString(domain.FlagProfile)
	if profile != "" {
		runCfg, err = rules.FilterToProfile(runCfg, profile)
		if err != nil {
			return err
		}
	}

	return output.WriteRunConfigJSON(cmd.OutOrStdout(), runCfg)
}
