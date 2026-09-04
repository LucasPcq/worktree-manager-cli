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
	"github.com/LucasPcq/wtm/internal/flow/run/seam"
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
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	ctx, err := loadURLContext(cmd, cwd)
	if err != nil {
		return err
	}

	// No wizard on any axis: `run url` answers from its flags or errors. Inside
	// curl $(wtm run url --job api) the substitution captures stdout but stdin is
	// still the terminal, so a form would open mid-substitution and hang the shell.
	resolved, err := resolveInputs(inputsParams{Args: args, Cwd: cwd, ProjectDir: ctx.config.ProjectDir})
	if err != nil {
		return err
	}
	entries := ctx.publishedIn(resolved.Dir)

	jobName, _ := cmd.Flags().GetString(domain.FlagJob)

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	if format == domain.OutputJSON {
		// --job narrows the document as it narrows the line: a caller asking for
		// one job's address must not have to find it in an array.
		if jobName != "" {
			entry, err := pickPublished(entries, jobName)
			if err != nil {
				return err
			}
			entries = []domain.JobURLEntry{entry}
		}
		return output.WriteJobURLsJSON(cmd.OutOrStdout(), entries)
	}

	entry, err := pickPublished(entries, jobName)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), entry.URL)
	return nil
}

// urlContext is what every address in this repository is computed from. It never
// contacts the daemon: a job's address is a property of its worktree's offset,
// known whether or not anything is running.
type urlContext struct {
	config shared.ConfigResult
	run    domain.RunConfig
	// proxyPort is zero under --raw, which asks for the job's own port instead of
	// the name the proxy serves it under.
	proxyPort int
}

func loadURLContext(cmd *cobra.Command, cwd string) (urlContext, error) {
	result, err := shared.LoadConfig(cmd, cwd)
	if err != nil {
		return urlContext{}, err
	}

	runCfg, err := config.LoadRun(result.StateDir)
	if err != nil {
		return urlContext{}, fmt.Errorf("load run config: %w", err)
	}
	if err := shared.RequireRunInitialized(runCfg); err != nil {
		return urlContext{}, err
	}

	proxyPort := process.PublicProxyPort(rules.ProxyPort(result.Config.Global))
	if raw, _ := cmd.Flags().GetBool(domain.FlagRaw); raw {
		proxyPort = 0
	}
	return urlContext{config: result, run: runCfg, proxyPort: proxyPort}, nil
}

// publishedIn lists the jobs reachable in one worktree. The worktree is what
// makes the addresses differ: its ordinal decides every port.
func (c urlContext) publishedIn(dir string) []domain.JobURLEntry {
	env := seam.JobEnv(seam.JobEnvParams{ProjectDir: c.config.ProjectDir, StateDir: c.config.StateDir, WorkDir: dir})
	offset, _ := strconv.Atoi(env[domain.EnvPortOffset])
	project := filepath.Base(c.config.ProjectDir)

	var entries []domain.JobURLEntry
	for _, job := range c.run.Jobs {
		ports := rules.JobPorts(rules.JobPortsParams{Ports: job.Ports, PortOffset: offset})
		url := rules.JobURL(rules.JobURLParams{
			Job:        job,
			Ports:      ports,
			Host:       rules.RouteHost(rules.RouteHostParams{Job: job, Worktree: env[domain.EnvWorktree], Project: project}),
			PublicPort: c.proxyPort,
		})
		if url == "" {
			continue
		}
		entries = append(entries, domain.JobURLEntry{Job: job.Name, URL: url})
	}
	return entries
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
