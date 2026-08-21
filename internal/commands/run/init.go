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
	"github.com/LucasPcq/wtm/internal/service/compose"
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
			"Ports declared in the selected compose files become per-worktree ports. A literal\n" +
			"host port (\"5432:5432\") binds the same port everywhere, so wtm offers to rewrite it\n" +
			"as \"${DB_PORT:-5432}:5432\" — the default keeps `docker compose up` working on its\n" +
			"own. Declining leaves the file untouched and declares no port for it.\n\n" +
			domain.ExperimentalRunNotice,
		Args: cobra.NoArgs,
		RunE: runRunInit,
	}
	cmd.Flags().Bool(domain.FlagNonInteractive, false, "Auto-generate from detection; never prompt")
	cmd.Flags().Bool(domain.FlagPatchCompose, false, "Rewrite literal host ports in the selected compose files to read a variable")
	cmd.Flags().Bool(domain.FlagLinkEnv, false, "Link the .env keys holding a declared port, so each worktree gets its own")
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
	patchCompose, _ := cmd.Flags().GetBool(domain.FlagPatchCompose)
	linkEnv, _ := cmd.Flags().GetBool(domain.FlagLinkEnv)
	interactive := !nonInteractive && term.IsTerminal(int(os.Stdin.Fd()))

	var detection domain.InitDetectionResult
	_ = components.RunLoading(components.LoadingParams{
		Message: "Detecting services…",
		Animate: interactive,
		Work: func() error {
			detection = detect.ProjectEnvironment(res.ProjectDir)
			detection.ComposeScans = compose.ScanAll(compose.ScanAllParams{
				ProjectDir: res.ProjectDir,
				Files:      detection.DockerComposeFiles,
			})
			return nil
		},
	})

	existing, err := runconfig.Load(res.StateDir)
	if err != nil {
		return fmt.Errorf("load run.toml: %w", err)
	}

	answers, err := resolveServicesAnswers(resolveServicesParams{
		Interactive:  interactive,
		ProjectDir:   res.ProjectDir,
		Detection:    detection,
		Existing:     existing,
		PatchCompose: patchCompose,
	})
	if errors.Is(err, domain.ErrUserAborted) {
		return nil
	}
	if err != nil {
		return err
	}

	plan := rules.PlanComposePorts(rules.PlanComposePortsParams{
		Scans: detection.ComposeScans,
		Files: answers.DockerComposeFiles,
		Patch: answers.PatchCompose,
	})
	outcome := rules.ResolveComposePorts(rules.ResolveComposePortsParams{
		Answers:        answers,
		PackageManager: detection.PackageManager,
		Existing:       existing,
		Plan:           plan,
		Unverifiable: compose.VerifyAll(compose.VerifyAllParams{
			ProjectDir: res.ProjectDir,
			ByFile:     plan.Patches,
		}),
	})

	if !rules.IsRunInitialized(outcome.Config) {
		output.Frame(cmd.OutOrStdout(), func() {
			output.Message(cmd.OutOrStdout(), "No docker-compose files or package scripts detected — nothing to configure automatically.")
			output.Message(cmd.OutOrStdout(), "Add jobs by hand with `wtm run job add`, then group them with `wtm run profile add`.")
			output.Blank(cmd.OutOrStdout())
			output.Message(cmd.OutOrStdout(), domain.ExperimentalRunNotice)
		})
		return nil
	}

	links := resolveEnvPortLinks(resolveEnvPortLinksParams{
		Interactive: interactive,
		LinkEnv:     linkEnv,
		ProjectDir:  res.ProjectDir,
		EnvFiles:    res.Config.Project.Env.Files,
		Config:      outcome.Config,
	})
	outcome.Config.EnvPorts = append(outcome.Config.EnvPorts, links...)

	// The rewrites come first: a compose templatized without run.toml behind it
	// keeps binding its defaults, while a run.toml declaring ports the compose
	// does not read would announce an isolation that is not there.
	if err := compose.PatchAll(compose.PatchAllParams{ProjectDir: res.ProjectDir, ByFile: outcome.Patches}); err != nil {
		return err
	}
	if err := runconfig.Save(runconfig.SaveParams{StateDir: res.StateDir, Config: outcome.Config}); err != nil {
		return err
	}

	runPath := filepath.Join(res.StateDir, domain.RunFileName)
	output.Frame(cmd.OutOrStdout(), func() {
		output.Success(cmd.OutOrStdout(), fmt.Sprintf("Configured run module → %s", runPath))
		if len(outcome.Merge.Added) > 0 {
			output.Message(cmd.OutOrStdout(), fmt.Sprintf("Jobs added: %s", strings.Join(outcome.Merge.Added, ", ")))
		}
		if len(outcome.Merge.Skipped) > 0 {
			output.Message(cmd.OutOrStdout(), fmt.Sprintf("Already present (kept): %s", strings.Join(outcome.Merge.Skipped, ", ")))
		}
		output.ComposePortsReport(cmd.OutOrStdout(), output.ComposePortsReportParams{
			Patched:    outcome.Patches,
			Written:    outcome.Written,
			Withheld:   outcome.Withheld,
			JobsByFile: composeJobsByFile(outcome.Config, answers.DockerComposeFiles),
			Dropped:    outcome.Dropped,
			Unreadable: outcome.Unreadable,
			Changed:    outcome.Changed,
			Orphaned:   outcome.Orphaned,
		})
		output.EnvPortLinksReport(cmd.OutOrStdout(), links, rules.EnvPortBases(outcome.Config))
		output.Blank(cmd.OutOrStdout())
		output.Message(cmd.OutOrStdout(), "Next: `wtm run up` to start · `wtm run job add` to add more")
		output.Blank(cmd.OutOrStdout())
		output.Message(cmd.OutOrStdout(), domain.ExperimentalRunNotice)
	})
	return nil
}

type resolveServicesParams struct {
	Interactive  bool
	ProjectDir   string
	Detection    domain.InitDetectionResult
	Existing     domain.RunConfig
	PatchCompose bool
}

// resolveServicesAnswers gathers the services selection either from the wizard
// (interactive) or straight from detection (non-interactive). On a re-run the
// wizard is pre-filled with what run.toml already declares so the subsequent
// merge is additive rather than a fresh overwrite.
func resolveServicesAnswers(params resolveServicesParams) (domain.InitProjectAnswers, error) {
	if !params.Interactive {
		return rules.AutoServicesAnswers(rules.AutoServicesAnswersParams{
			Detection:    params.Detection,
			PatchCompose: params.PatchCompose,
		}), nil
	}

	var prefill *initwizard.SectionPrefill
	if rules.IsRunInitialized(params.Existing) {
		prefill = &initwizard.SectionPrefill{
			DockerFiles:   rules.DockerFilesConfigured(params.Existing, params.Detection.DockerComposeFiles),
			ScriptIndices: rules.ScriptsConfigured(params.Existing, params.Detection.PackageScripts, params.Detection.PackageManager),
		}
	}

	return initwizard.RunServicesWizard(initwizard.ServicesWizardParams{
		ProjectDir:   params.ProjectDir,
		Detection:    params.Detection,
		Existing:     params.Existing,
		Prefill:      prefill,
		PatchCompose: params.PatchCompose,
	})
}

// composeJobsByFile turns a withheld port's fix into a command to paste rather
// than a placeholder.
func composeJobsByFile(cfg domain.RunConfig, files []string) map[string]string {
	jobs := make(map[string]string, len(files))
	for _, file := range files {
		if job := rules.ComposeJobName(rules.ComposeJobNameParams{Config: cfg, File: file}); job != "" {
			jobs[file] = job
		}
	}
	return jobs
}

type resolveEnvPortLinksParams struct {
	Interactive bool
	// LinkEnv is --link-env already given on the command line: the links are
	// authorized, so nothing is asked.
	LinkEnv    bool
	ProjectDir string
	EnvFiles   []domain.EnvFile
	Config     domain.RunConfig
}

// resolveEnvPortLinks offers the .env keys whose value holds one of the ports
// this run just settled. Nothing is inferred: a link is written from --link-env
// or from an explicit confirmation, never because the detection found a match.
func resolveEnvPortLinks(params resolveEnvPortLinksParams) []domain.EnvPortLink {
	candidates := detect.EnvPortCandidates(detect.EnvPortCandidatesParams{
		ProjectDir: params.ProjectDir,
		Files:      params.EnvFiles,
		Bases:      rules.EnvPortBases(params.Config),
		Existing:   params.Config.EnvPorts,
	})
	if len(candidates) == 0 {
		return nil
	}
	if params.LinkEnv {
		return candidates
	}
	if !params.Interactive {
		return nil
	}

	// The candidates go in the prompt's own description rather than a block
	// printed before it: the prompt renders on stderr inside its own frame, and a
	// command frames its stdout exactly once.
	confirmed, err := components.RunStandaloneConfirm(components.NewConfirm(components.NewConfirmParams{
		Title: domain.EnvPortLinkConfirm,
		Description: strings.Join(append(
			[]string{domain.EnvPortLinkDescription, ""},
			rules.EnvPortLinkLines(candidates, rules.EnvPortBases(params.Config))...), "\n"),
		DefaultYes: true,
	}))
	if err != nil || !confirmed {
		return nil
	}
	return candidates
}
