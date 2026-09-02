package run

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
)

func newURLCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdURL + " [worktree]",
		Short: "Print where a job is reachable in a worktree",
		Long:  "Write a job's URL on stdout and nothing else, for $(…). [worktree] defaults to the current one, and no picker ever opens here — an ambiguity is an error naming --job. --raw prints the job's own port instead of its name, which every OS resolves and no proxy has to serve.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runURL,
	}
	shared.AddJobFlag(cmd, "Job whose URL to print (required when several jobs publish one)")
	cmd.Flags().Bool(domain.FlagRaw, false, "Print the direct http://localhost:<port> address")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runURL(cmd *cobra.Command, args []string) error {
	entries, err := publishedJobs(cmd, args, false)
	if err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	if format == domain.OutputJSON {
		return output.WriteJobURLsJSON(cmd.OutOrStdout(), entries)
	}

	jobName, _ := cmd.Flags().GetString(domain.FlagJob)
	entry, err := pickPublished(entries, jobName)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), entry.URL)
	return nil
}

// publishedJobs resolves the target worktree's ports, then keeps the jobs that
// declare a url. It never contacts the daemon: a job's address is a property of
// the worktree's offset, known whether or not anything is running.
func publishedJobs(cmd *cobra.Command, args []string, pick bool) ([]domain.JobURLEntry, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}

	result, err := shared.LoadConfig(cmd, cwd)
	if err != nil {
		return nil, err
	}

	runCfg, err := config.LoadRun(result.StateDir)
	if err != nil {
		return nil, fmt.Errorf("load run config: %w", err)
	}
	if err := shared.RequireRunInitialized(runCfg); err != nil {
		return nil, err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	tgt, err := resolveTarget(targetParams{
		Args:        args,
		Cwd:         cwd,
		ProjectDir:  result.ProjectDir,
		Interactive: isTTY() && rules.IsHumanFormat(format),
		Pick:        pick,
	})
	if err != nil {
		return nil, err
	}

	env := jobEnv(jobEnvParams{ProjectDir: result.ProjectDir, StateDir: result.StateDir, Dir: tgt.Dir})
	offset, _ := strconv.Atoi(env[domain.EnvPortOffset])

	raw, _ := cmd.Flags().GetBool(domain.FlagRaw)
	proxyPort := process.PublicProxyPort(rules.ProxyPort(result.Config.Global))
	if raw {
		proxyPort = 0
	}
	project := filepath.Base(result.ProjectDir)

	var entries []domain.JobURLEntry
	for _, job := range runCfg.Jobs {
		ports := rules.JobPorts(rules.JobPortsParams{Ports: job.Ports, PortOffset: offset})
		url := rules.JobURL(rules.JobURLParams{
			Job:        job,
			Ports:      ports,
			Host:       rules.RouteHost(rules.RouteHostParams{Job: job, Worktree: env[domain.EnvWorktree], Project: project}),
			PublicPort: proxyPort,
		})
		if url == "" {
			continue
		}
		entries = append(entries, domain.JobURLEntry{Job: job.Name, URL: url})
	}
	return entries, nil
}

// pickPublished resolves which job the caller meant without ever offering a
// picker: `run url` is a machine surface, so an ambiguity is an error naming the
// flag, not a prompt.
func pickPublished(entries []domain.JobURLEntry, jobName string) (domain.JobURLEntry, error) {
	if len(entries) == 0 {
		return domain.JobURLEntry{}, domain.ErrJobNonePublished
	}
	if jobName != "" {
		return findPublished(entries, jobName)
	}
	if len(entries) > 1 {
		return domain.JobURLEntry{}, fmt.Errorf("%w — %s", domain.ErrJobAmbiguous, publishedNames(entries))
	}
	return entries[0], nil
}

func findPublished(entries []domain.JobURLEntry, name string) (domain.JobURLEntry, error) {
	for _, entry := range entries {
		if entry.Job == name {
			return entry, nil
		}
	}
	return domain.JobURLEntry{}, fmt.Errorf("job %q publishes no url — these do: %s", name, publishedNames(entries))
}

func publishedNames(entries []domain.JobURLEntry) string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Job)
	}
	return strings.Join(names, domain.RunURLListSep)
}
