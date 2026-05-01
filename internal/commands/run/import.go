package run

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
)

// newImportCmd creates the wtm run import subcommand.
func newImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdImport + " [file]",
		Short: "Import a JSON run config into run.toml",
		Long: `Read a JSON run config payload from a file (or stdin) and merge it into run.toml.

Pass "-" or omit the argument to read from stdin.

By default, new jobs and profiles are appended; duplicates are skipped with a warning.
Use --replace --force to overwrite the file entirely.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runImport,
	}
	cmd.Flags().Bool(domain.FlagReplace, false, "Overwrite run.toml entirely (requires --force)")
	cmd.Flags().Bool(domain.FlagForce, false, "Confirm destructive --replace")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runImport(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, ok := shared.LoadConfig(cmd, dir)
	if !ok {
		return nil
	}

	replace, _ := cmd.Flags().GetBool(domain.FlagReplace)
	force, _ := cmd.Flags().GetBool(domain.FlagForce)
	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	if replace && !force {
		return fmt.Errorf("--replace is destructive: pass --force to confirm")
	}

	data, err := readImportSource(args)
	if err != nil {
		return err
	}

	var incoming domain.RunConfig
	if err := json.Unmarshal(data, &incoming); err != nil {
		return fmt.Errorf("parse JSON: %w", err)
	}

	_, errs := rules.ValidateRun(incoming)
	if len(errs) > 0 {
		return fmt.Errorf("invalid run config:\n  %s", strings.Join(errs, "\n  "))
	}

	if replace {
		return writeAndReport(cmd, result.StateDir, incoming, rules.MergeResult{}, format, true)
	}

	existing, err := config.LoadRun(result.StateDir)
	if err != nil {
		return fmt.Errorf("load existing run config: %w", err)
	}

	merged, mergeResult := rules.MergeRunConfigs(existing, incoming)
	return writeAndReport(cmd, result.StateDir, merged, mergeResult, format, false)
}

func writeAndReport(cmd *cobra.Command, stateDir string, cfg domain.RunConfig, mergeResult rules.MergeResult, format string, replace bool) error {
	if err := config.WriteRun(config.WriteRunParams{
		StateDir: stateDir,
		Config:   cfg,
		Force:    true,
	}); err != nil {
		return fmt.Errorf("write run config: %w", err)
	}

	ir := output.ImportResult{
		Added:   mergeResult.Added,
		Skipped: mergeResult.Skipped,
	}
	if replace {
		for _, j := range cfg.Jobs {
			ir.Added = append(ir.Added, j.Name)
		}
	}

	if format == domain.OutputJSON {
		return output.WriteImportResultJSON(cmd.OutOrStdout(), ir)
	}
	return output.WriteImportResultText(cmd.OutOrStdout(), ir)
}

func readImportSource(args []string) ([]byte, error) {
	if len(args) == 0 || args[0] == "-" {
		return io.ReadAll(os.Stdin)
	}
	data, err := os.ReadFile(args[0])
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", args[0], err)
	}
	return data, nil
}
