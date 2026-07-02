package run

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/detect"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
	"github.com/LucasPcq/wtm/internal/tui/components"
	initwizard "github.com/LucasPcq/wtm/internal/tui/inittui"
)

// newInitCmd creates the wtm run init subcommand — the dedicated entry point
// that configures the (experimental) run module, kept out of the global
// `wtm init` wizard so users who never touch `run` aren't bothered by it.
func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdInit,
		Short: "Configure the run module (services & tasks) for this repo",
		Long: "Set up run.toml by detecting docker-compose files and package.json scripts and turning\n" +
			"the selected ones into jobs.\n\n" +
			"In a TTY, opens a wizard to pick which ones to include; non-interactively (or piped),\n" +
			"auto-generates from detection. Re-running merges new selections into the existing\n" +
			"run.toml without overwriting what's already there.\n\n" +
			domain.ExperimentalRunNotice,
		Args: cobra.NoArgs,
		RunE: runRunInit,
	}
	cmd.Flags().Bool(domain.FlagNonInteractive, false, "Auto-generate from detection; never prompt")
	return cmd
}

func runRunInit(cmd *cobra.Command, _ []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	res, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	nonInteractive, _ := cmd.Flags().GetBool(domain.FlagNonInteractive)
	interactive := !nonInteractive && term.IsTerminal(int(os.Stdin.Fd()))

	var detection domain.InitDetectionResult
	_ = components.RunLoading(components.LoadingParams{
		Message: "Detecting services…",
		Animate: interactive,
		Work:    func() error { detection = detect.ProjectEnvironment(res.ProjectDir); return nil },
	})

	existing, err := runconfig.Load(res.StateDir)
	if err != nil {
		return fmt.Errorf("load run.toml: %w", err)
	}

	answers, err := resolveServicesAnswers(resolveServicesParams{
		Interactive: interactive,
		ProjectDir:  res.ProjectDir,
		Detection:   detection,
		Existing:    existing,
	})
	if errors.Is(err, domain.ErrUserAborted) {
		return nil
	}
	if err != nil {
		return err
	}

	built := rules.BuildInitRunConfig(answers, detection.PackageManager)
	merged, mergeResult := rules.MergeRunConfigs(existing, built)

	if !rules.IsRunInitialized(merged) {
		output.Frame(cmd.OutOrStdout(), func() {
			output.Message(cmd.OutOrStdout(), "No docker-compose files or package scripts detected — nothing to configure automatically.")
			output.Message(cmd.OutOrStdout(), "Add jobs by hand with `wtm run job add`, then group them with `wtm run profile add`.")
			output.Blank(cmd.OutOrStdout())
			output.Message(cmd.OutOrStdout(), domain.ExperimentalRunNotice)
		})
		return nil
	}

	if err := runconfig.Save(runconfig.SaveParams{StateDir: res.StateDir, Config: merged}); err != nil {
		return err
	}

	runPath := filepath.Join(res.StateDir, domain.RunFileName)
	output.Frame(cmd.OutOrStdout(), func() {
		output.Success(cmd.OutOrStdout(), fmt.Sprintf("Configured run module → %s", runPath))
		if len(mergeResult.Added) > 0 {
			output.Message(cmd.OutOrStdout(), fmt.Sprintf("Jobs added: %s", strings.Join(mergeResult.Added, ", ")))
		}
		if len(mergeResult.Skipped) > 0 {
			output.Message(cmd.OutOrStdout(), fmt.Sprintf("Already present (kept): %s", strings.Join(mergeResult.Skipped, ", ")))
		}
		output.Blank(cmd.OutOrStdout())
		output.Message(cmd.OutOrStdout(), "Next: `wtm run up` to start · `wtm run job add` to add more")
		output.Blank(cmd.OutOrStdout())
		output.Message(cmd.OutOrStdout(), domain.ExperimentalRunNotice)
	})
	return nil
}

// resolveServicesParams groups the inputs for resolveServicesAnswers.
type resolveServicesParams struct {
	Interactive bool
	ProjectDir  string
	Detection   domain.InitDetectionResult
	Existing    domain.RunConfig
}

// resolveServicesAnswers gathers the services selection either from the wizard
// (interactive) or straight from detection (non-interactive). On a re-run the
// wizard is pre-filled with what run.toml already declares so the subsequent
// merge is additive rather than a fresh overwrite.
func resolveServicesAnswers(params resolveServicesParams) (domain.InitProjectAnswers, error) {
	if !params.Interactive {
		return rules.AutoServicesAnswers(params.Detection), nil
	}

	var prefill *initwizard.SectionPrefill
	if rules.IsRunInitialized(params.Existing) {
		prefill = &initwizard.SectionPrefill{
			DockerFiles:   rules.DockerFilesConfigured(params.Existing, params.Detection.DockerComposeFiles),
			ScriptIndices: rules.ScriptsConfigured(params.Existing, params.Detection.PackageScripts, params.Detection.PackageManager),
		}
	}

	return initwizard.RunServicesWizard(params.ProjectDir, params.Detection, prefill)
}
