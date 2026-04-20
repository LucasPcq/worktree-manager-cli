package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
)

// newRunImportCmd creates the wtm run import subcommand.
func newRunImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import [file]",
		Short: "Import a JSON run config into .wtm/run.toml",
		Long: `Read a JSON run config payload from a file (or stdin) and merge it into .wtm/run.toml.

Pass "-" or omit the argument to read from stdin.

By default, new jobs and profiles are appended; duplicates are skipped with a warning.
Use --replace --force to overwrite the file entirely.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runRunImport,
	}
	cmd.Flags().Bool(domain.FlagReplace, false, "Overwrite .wtm/run.toml entirely (requires --force)")
	cmd.Flags().Bool(domain.FlagForce, false, "Confirm destructive --replace")
	cmd.Flags().String(domain.FlagOutput, domain.OutputText, "Output format: text or json")
	return cmd
}

func runRunImport(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, ok := loadConfig(cmd, dir)
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

	_, errs := config.ValidateRun(incoming)
	if len(errs) > 0 {
		return fmt.Errorf("invalid run config:\n  %s", strings.Join(errs, "\n  "))
	}

	if replace {
		return writeAndReport(cmd, result.ProjectDir, incoming, config.MergeResult{}, format, true)
	}

	existing, err := config.LoadRun(result.ProjectDir)
	if err != nil {
		return fmt.Errorf("load existing run config: %w", err)
	}

	merged, mergeResult := config.MergeRunConfigs(existing, incoming)
	return writeAndReport(cmd, result.ProjectDir, merged, mergeResult, format, false)
}

func writeAndReport(cmd *cobra.Command, projectDir string, cfg domain.RunConfig, mergeResult config.MergeResult, format string, replace bool) error {
	if err := config.WriteRun(config.WriteRunParams{
		ProjectDir: projectDir,
		Config:     cfg,
		Force:      true,
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
