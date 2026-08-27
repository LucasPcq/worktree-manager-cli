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
		Use:   domain.CmdOpen + " [job]",
		Short: "Open a job's URL in the browser",
		Long:  "Hand a job's URL to the desktop's own opener. Naming the job is required outside a fully interactive run — a picker never runs under a pipe or --output json.",
		RunE:  runOpen,
	}
	cmd.Flags().Bool(domain.FlagRaw, false, "Open the direct http://localhost:<port> address")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runOpen(cmd *cobra.Command, args []string) error {
	entries, err := publishedJobs(cmd)
	if err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	interactive := isTTY() && rules.IsHumanFormat(format)

	entry, err := pickPublishedInteractive(pickInteractiveParams{
		Entries:     entries,
		Args:        args,
		Interactive: interactive,
	})
	if err != nil {
		return err
	}
	return openInBrowser(entry.URL)
}

type pickInteractiveParams struct {
	Entries     []domain.JobURLEntry
	Args        []string
	Interactive bool
}

// pickPublishedInteractive adds the one thing `run url` refuses: a picker, and
// only when the run is fully interactive.
func pickPublishedInteractive(params pickInteractiveParams) (domain.JobURLEntry, error) {
	if !params.Interactive || len(params.Args) > 0 || len(params.Entries) < 2 {
		return pickPublished(params.Entries, params.Args)
	}
	return runpicker.RunURLPicker(params.Entries)
}
