package run

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/integration"
)

// openInBrowser is a variable so a test can watch what a run would have opened
// without a browser window appearing.
var openInBrowser = integration.OpenURL

func askableURLs(entries []domain.JobURLEntry) []domain.JobURLEntry {
	if len(entries) < 2 {
		return nil
	}
	return entries
}

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
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	ctx, err := loadURLContext(cmd, cwd)
	if err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	jobName, _ := cmd.Flags().GetString(domain.FlagJob)

	resolved, err := resolveInputs(inputsParams{
		Args:        args,
		Cwd:         cwd,
		ProjectDir:  ctx.config.ProjectDir,
		Interactive: isTTY() && rules.IsHumanFormat(format),
		Pick:        true,
		Second: secondAxis{
			Given: jobName,
			// A single published job is the answer, not a question.
			URLs: askableURLs(ctx.publishedIn(worktreeRoot(cwd))),
			// The addresses depend on the worktree picked one step earlier, so the
			// list is rebuilt rather than shown with the ports of wherever the
			// command was launched.
			ResolveURLs: ctx.publishedIn,
		},
	})
	if err != nil {
		return err
	}

	entry, err := pickPublished(ctx.publishedIn(resolved.Dir), resolved.Second)
	if err != nil {
		return err
	}
	return openInBrowser(entry.URL)
}
