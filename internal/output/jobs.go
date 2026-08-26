package output

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

// JobActionResult is a single job's outcome emitted by `run *` commands.
type JobActionResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"`            // "started", "stopped", "done", "error", "added", "removed"
	Message string `json:"message,omitempty"` // error detail when Status == "error"
}

// ProfileActionResult is a single profile's outcome emitted by `run profile` commands.
type ProfileActionResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"`            // "added", "removed"
	Message string `json:"message,omitempty"` // error detail when Status == "error"
}

// PRCheckoutJSON is the payload emitted by `checkout --output json`.
type PRCheckoutJSON struct {
	Number int    `json:"number"`
	Branch string `json:"branch"`
	Path   string `json:"path"`
	Author string `json:"author"`
	URL    string `json:"url"`
	Draft  bool   `json:"is_draft"`
	// ExistingBranch reports that a local branch of that name already existed and
	// was checked out as-is, instead of being created from origin.
	ExistingBranch bool `json:"existing_branch"`
	// OriginState is the reused branch's divergence from origin, using the same
	// labels as `list` and `tree`. Empty when the branch was created.
	OriginState string `json:"origin_state,omitempty"`
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

// WriteProfileResultJSON writes a single profile outcome.
func WriteProfileResultJSON(w io.Writer, result ProfileActionResult) error {
	return encodeJSON(w, result)
}

// WritePRCheckoutJSON writes the payload for `wtm checkout`.
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

// WriteImportResultText writes the import result as human-readable status lines,
// using the shared icon vocabulary: ✓ for imported jobs, = for duplicates left
// untouched. It emits a raw body; the caller's frame owns the outer padding.
func WriteImportResultText(w io.Writer, result ImportResult) {
	if len(result.Added) == 0 && len(result.Skipped) == 0 {
		Message(w, "Nothing to import.")
		return
	}
	for _, name := range result.Added {
		Success(w, fmt.Sprintf("Imported %s", name))
	}
	for _, name := range result.Skipped {
		Unchanged(w, fmt.Sprintf("%s already present", name))
	}
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

// WriteJobsJSON writes the JSON array of jobs for `run job list`.
func WriteJobsJSON(w io.Writer, jobs []domain.JobConfig) error {
	if jobs == nil {
		jobs = []domain.JobConfig{}
	}
	return encodeJSON(w, jobs)
}

// WriteProfilesJSON writes the JSON array of profiles for `run profile list`.
func WriteProfilesJSON(w io.Writer, profiles []domain.ProfileConfig) error {
	if profiles == nil {
		profiles = []domain.ProfileConfig{}
	}
	return encodeJSON(w, profiles)
}

// WriteRunningJobsJSON writes the JSON payload for `run ps`.
func WriteRunningJobsJSON(w io.Writer, jobs []domain.JobInfo) error {
	if jobs == nil {
		jobs = []domain.JobInfo{}
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

// FormatRunningJobs renders a table of running (or recently running) jobs. It
// returns a raw body with no outer blank lines; the caller's frame owns the
// outer vertical padding.
func FormatRunningJobs(jobs []domain.JobInfo) string {
	if len(jobs) == 0 {
		return Indent + "No jobs running.\n"
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
