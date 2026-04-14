package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/schemas"
)

// NewSchemaCmd creates the wtm schema command group — exposes the JSON
// Schema files bundled with this binary so editors (Taplo, etc.) can offer
// autocomplete and validation on TOML config files.
func NewSchemaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "schema",
		Short:   "Inspect or extract bundled JSON Schemas",
		Long:    "JSON Schemas describe the structure of wtm's TOML config files.\nUse `wtm schema dump` to write them to .wtm/schemas/ so editors can pick them up via the `#:schema` directive.",
		GroupID: domain.CmdGroupSetup,
	}
	cmd.AddCommand(newSchemaDumpCmd())
	return cmd
}

func newSchemaDumpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dump",
		Short: "Write embedded schemas to .wtm/schemas/ (or ~/.config/wtm/schemas with --global)",
		Long:  "Extract every JSON Schema bundled with this wtm binary so editors can resolve the `#:schema` directives in your TOML files.\nProject schemas land in .wtm/schemas/. Use --global to write the global schema next to ~/.config/wtm/config.toml.",
		RunE:  runSchemaDump,
	}
	cmd.Flags().Bool(domain.FlagGlobal, false, "Write the global config schema instead of the project ones")
	return cmd
}

func runSchemaDump(cmd *cobra.Command, _ []string) error {
	global, _ := cmd.Flags().GetBool(domain.FlagGlobal)

	if global {
		dir, err := os.UserConfigDir()
		if err != nil {
			return fmt.Errorf("locate user config dir: %w", err)
		}
		schemaDir := filepath.Join(dir, domain.GlobalConfigDir, domain.SchemasDirName)
		written, err := writeSchemas(schemaDir, []schemas.Schema{schemas.Global})
		if err != nil {
			return err
		}
		output.Blank(cmd.OutOrStdout())
		for _, p := range written {
			output.Success(cmd.OutOrStdout(), fmt.Sprintf("Wrote %s", p))
		}
		output.Blank(cmd.OutOrStdout())
		return nil
	}

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	root, err := projectRoot(wd)
	if err != nil {
		return err
	}
	schemaDir := filepath.Join(root, domain.ProjectDirName, domain.SchemasDirName)
	written, err := writeSchemas(schemaDir, []schemas.Schema{schemas.Project, schemas.Run})
	if err != nil {
		return err
	}
	output.Blank(cmd.OutOrStdout())
	for _, p := range written {
		output.Success(cmd.OutOrStdout(), fmt.Sprintf("Wrote %s", p))
	}
	output.Blank(cmd.OutOrStdout())
	return nil
}

// writeSchemas extracts the given schemas into dir, creating it if needed.
// Returns the list of paths written (relative to dir's parent for display).
func writeSchemas(dir string, list []schemas.Schema) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create %s: %w", dir, err)
	}
	written := make([]string, 0, len(list))
	for _, s := range list {
		path := filepath.Join(dir, s.Filename())
		if err := os.WriteFile(path, s.Bytes(), 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", path, err)
		}
		written = append(written, path)
	}
	return written, nil
}
