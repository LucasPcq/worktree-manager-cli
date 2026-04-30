package output

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/styles"
)

// JobActionResult is a single job's outcome emitted by `run *` commands.
type JobActionResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"`            // "started", "stopped", "done", "error"
	Message string `json:"message,omitempty"` // error detail when Status == "error"
}

// PRCheckoutJSON is the payload emitted by `pr checkout --output json`.
type PRCheckoutJSON struct {
	Number int    `json:"number"`
	Branch string `json:"branch"`
	Path   string `json:"path"`
}

// WriteJobResultsJSON writes the JSON array describing each job outcome.
func WriteJobResultsJSON(w io.Writer, results []JobActionResult) error {
	if results == nil {
		results = []JobActionResult{}
	}
	return encodeJSON(w, results)
}

// WriteJobResultJSON writes a single job outcome (start/stop single job).
func WriteJobResultJSON(w io.Writer, result JobActionResult) error {
	return encodeJSON(w, result)
}

// WritePRCheckoutJSON writes the payload for `pr checkout`.
func WritePRCheckoutJSON(w io.Writer, payload PRCheckoutJSON) error {
	return encodeJSON(w, payload)
}

// ImportResult describes the outcome of a wtm run import operation.
type ImportResult struct {
	Added   []string `json:"added"`
	Skipped []string `json:"skipped"`
}

// WriteImportResultJSON writes the import result as JSON.
func WriteImportResultJSON(w io.Writer, result ImportResult) error {
	if result.Added == nil {
		result.Added = []string{}
	}
	if result.Skipped == nil {
		result.Skipped = []string{}
	}
	return encodeJSON(w, result)
}

// WriteImportResultText writes the import result as human-readable text.
func WriteImportResultText(w io.Writer, result ImportResult) error {
	if len(result.Added) > 0 {
		if _, err := fmt.Fprintf(w, "%sAdded: %s\n", Indent, strings.Join(result.Added, ", ")); err != nil {
			return err
		}
	}
	if len(result.Skipped) > 0 {
		if _, err := fmt.Fprintf(w, "%sSkipped (duplicate): %s\n", Indent, strings.Join(result.Skipped, ", ")); err != nil {
			return err
		}
	}
	if len(result.Added) == 0 && len(result.Skipped) == 0 {
		_, err := fmt.Fprintf(w, "%sNothing to import.\n", Indent)
		return err
	}
	return nil
}

// WriteRunConfigJSON writes the JSON payload for `run list`.
func WriteRunConfigJSON(w io.Writer, cfg domain.RunConfig) error {
	if cfg.Jobs == nil {
		cfg.Jobs = []domain.JobConfig{}
	}
	if cfg.Profiles == nil {
		cfg.Profiles = []domain.ProfileConfig{}
	}
	return encodeJSON(w, cfg)
}

// WriteRunningJobsJSON writes the JSON payload for `run ps`.
func WriteRunningJobsJSON(w io.Writer, jobs []process.JobInfo) error {
	if jobs == nil {
		jobs = []process.JobInfo{}
	}
	return encodeJSON(w, jobs)
}

// FormatRunConfig renders jobs + profiles as two aligned text tables.
func FormatRunConfig(cfg domain.RunConfig) string {
	var b strings.Builder

	if len(cfg.Profiles) > 0 {
		b.WriteString(Indent)
		b.WriteString(styles.Bold.Render("PROFILES"))
		b.WriteString("\n")
		nameWidth := 0
		for _, p := range cfg.Profiles {
			if len(p.Name) > nameWidth {
				nameWidth = len(p.Name)
			}
		}
		for _, p := range cfg.Profiles {
			tag := ""
			if p.Default {
				tag = styles.Success.Render("default")
			}
			jobs := styles.Muted.Render(strings.Join(p.Jobs, ", "))
			b.WriteString(fmt.Sprintf("%s%-*s  %-9s  %s\n", Indent, nameWidth, p.Name, tag, jobs))
		}
	}

	if len(cfg.Jobs) > 0 {
		if len(cfg.Profiles) > 0 {
			b.WriteString("\n")
		}
		b.WriteString(Indent)
		b.WriteString(styles.Bold.Render("JOBS"))
		b.WriteString("\n")
		nameWidth, kindWidth := 0, 0
		for _, j := range cfg.Jobs {
			if len(j.Name) > nameWidth {
				nameWidth = len(j.Name)
			}
			if len(string(j.Kind)) > kindWidth {
				kindWidth = len(string(j.Kind))
			}
		}
		for _, j := range cfg.Jobs {
			cmd := styles.Muted.Render(j.Cmd)
			kind := styles.Muted.Render(fmt.Sprintf("%-*s", kindWidth, string(j.Kind)))
			b.WriteString(fmt.Sprintf("%s%-*s  %s  %s\n", Indent, nameWidth, j.Name, kind, cmd))
		}
	}

	if len(cfg.Profiles) == 0 && len(cfg.Jobs) == 0 {
		b.WriteString(Indent)
		b.WriteString("No jobs or profiles defined in run.toml.\n")
	}

	return b.String()
}

// FormatRunningJobs renders a table of running (or recently running) jobs.
func FormatRunningJobs(jobs []process.JobInfo) string {
	if len(jobs) == 0 {
		return "\n" + Indent + "No jobs running.\n\n"
	}

	nameW, kindW, statusW, pidW := len("NAME"), len("KIND"), len("STATUS"), len("PID")
	for _, j := range jobs {
		if len(j.Name) > nameW {
			nameW = len(j.Name)
		}
		if len(string(j.Kind)) > kindW {
			kindW = len(string(j.Kind))
		}
		if len(string(j.Status)) > statusW {
			statusW = len(string(j.Status))
		}
		pid := strconv.Itoa(j.PID)
		if len(pid) > pidW {
			pidW = len(pid)
		}
	}

	var b strings.Builder
	header := fmt.Sprintf("%s%-*s  %-*s  %-*s  %-*s  %s\n",
		Indent,
		nameW, "NAME",
		kindW, "KIND",
		statusW, "STATUS",
		pidW, "PID",
		"WORKTREE",
	)
	b.WriteString(styles.Muted.Render(header))

	for _, j := range jobs {
		status := styleJobStatus(j.Status)
		pid := ""
		if j.PID != 0 {
			pid = strconv.Itoa(j.PID)
		}
		line := fmt.Sprintf("%s%-*s  %-*s  %-*s  %-*s  %s\n",
			Indent,
			nameW, j.Name,
			kindW, string(j.Kind),
			statusW+ansiOverhead(status), status,
			pidW, pid,
			styles.Muted.Render(j.WorkDir),
		)
		b.WriteString(line)
	}

	return b.String()
}

func styleJobStatus(status domain.JobStatus) string {
	switch status {
	case domain.JobStatusRunning:
		return styles.Success.Render(string(status))
	case domain.JobStatusCrashed:
		return styles.Warning.Render(string(status))
	default:
		return styles.Muted.Render(string(status))
	}
}
