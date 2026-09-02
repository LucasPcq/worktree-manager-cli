package run

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/integration"
	"github.com/LucasPcq/wtm/internal/tui/runpicker"
)

// openInBrowser is a variable so a test can watch what a run would have opened
// without a browser window appearing.
var openInBrowser = integration.OpenURL

func newOpenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdOpen + " [worktree]",
		Short: "Open a job's URL in the browser",
		Long:  "Hand a job's URL to the desktop's own opener. [worktree] defaults to the current one, and is picked interactively when there is a terminal. Naming the job with --job is required outside a fully interactive run — a picker never runs under a pipe or --output json.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runOpen,
	}
	shared.AddJobFlag(cmd, "Job whose URL to open (required outside a fully interactive run)")
	cmd.Flags().Bool(domain.FlagRaw, false, "Open the direct http://localhost:<port> address")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runOpen(cmd *cobra.Command, args []string) error {
	entries, err := publishedJobs(cmd, args, true)
	if err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	interactive := isTTY() && rules.IsHumanFormat(format)
	jobName, _ := cmd.Flags().GetString(domain.FlagJob)

	entry, err := pickPublishedInteractive(pickInteractiveParams{
		Entries:     entries,
		JobName:     jobName,
		Interactive: interactive,
	})
	if err != nil {
		return err
	}
	return openInBrowser(entry.URL)
}

type pickInteractiveParams struct {
	Entries     []domain.JobURLEntry
	JobName     string
	Interactive bool
}

// pickPublishedInteractive adds the one thing `run url` refuses: a picker, and
// only when the run is fully interactive.
func pickPublishedInteractive(params pickInteractiveParams) (domain.JobURLEntry, error) {
	if !params.Interactive || params.JobName != "" || len(params.Entries) < 2 {
		return pickPublished(params.Entries, params.JobName)
	}
	return runpicker.RunURLPicker(params.Entries)
}
