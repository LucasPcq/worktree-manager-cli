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

// ServiceActionResult is a single service's outcome emitted by svc commands.
type ServiceActionResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"`            // "started", "stopped", "error"
	Message string `json:"message,omitempty"` // error detail when Status == "error"
}

// PRCheckoutJSON is the payload emitted by `pr checkout --output json`.
type PRCheckoutJSON struct {
	Number int    `json:"number"`
	Branch string `json:"branch"`
	Path   string `json:"path"`
}

// WriteServiceResultsJSON writes the JSON array describing each service outcome.
func WriteServiceResultsJSON(w io.Writer, results []ServiceActionResult) error {
	if results == nil {
		results = []ServiceActionResult{}
	}
	return encodeJSON(w, results)
}

// WriteServiceResultJSON writes a single service outcome (start/stop single service).
func WriteServiceResultJSON(w io.Writer, result ServiceActionResult) error {
	return encodeJSON(w, result)
}

// WritePRCheckoutJSON writes the payload for `pr checkout`.
func WritePRCheckoutJSON(w io.Writer, payload PRCheckoutJSON) error {
	return encodeJSON(w, payload)
}

// WriteServicesConfigJSON writes the JSON payload for `svc list`.
func WriteServicesConfigJSON(w io.Writer, cfg domain.ServicesConfig) error {
	if cfg.Services == nil {
		cfg.Services = []domain.ServiceConfig{}
	}
	if cfg.Profiles == nil {
		cfg.Profiles = []domain.ProfileConfig{}
	}
	return encodeJSON(w, cfg)
}

// WriteRunningServicesJSON writes the JSON payload for `svc ps`.
func WriteRunningServicesJSON(w io.Writer, services []process.ServiceInfo) error {
	if services == nil {
		services = []process.ServiceInfo{}
	}
	return encodeJSON(w, services)
}

// FormatServicesConfig renders services + profiles as two aligned text tables.
func FormatServicesConfig(cfg domain.ServicesConfig) string {
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
			services := styles.Muted.Render(strings.Join(p.Services, ", "))
			b.WriteString(fmt.Sprintf("%s%-*s  %-9s  %s\n", Indent, nameWidth, p.Name, tag, services))
		}
	}

	if len(cfg.Services) > 0 {
		if len(cfg.Profiles) > 0 {
			b.WriteString("\n")
		}
		b.WriteString(Indent)
		b.WriteString(styles.Bold.Render("SERVICES"))
		b.WriteString("\n")
		nameWidth := 0
		for _, s := range cfg.Services {
			if len(s.Name) > nameWidth {
				nameWidth = len(s.Name)
			}
		}
		for _, s := range cfg.Services {
			cmd := styles.Muted.Render(s.Cmd)
			b.WriteString(fmt.Sprintf("%s%-*s  %s\n", Indent, nameWidth, s.Name, cmd))
		}
	}

	if len(cfg.Profiles) == 0 && len(cfg.Services) == 0 {
		b.WriteString(Indent)
		b.WriteString("No services or profiles defined in .wtm/services.toml.\n")
	}

	return b.String()
}

// FormatRunningServices renders a table of running (or recently running) services.
func FormatRunningServices(services []process.ServiceInfo) string {
	if len(services) == 0 {
		return "\n" + Indent + "No services running.\n\n"
	}

	nameW, statusW, pidW := len("NAME"), len("STATUS"), len("PID")
	for _, s := range services {
		if len(s.Name) > nameW {
			nameW = len(s.Name)
		}
		if len(string(s.Status)) > statusW {
			statusW = len(string(s.Status))
		}
		pid := strconv.Itoa(s.PID)
		if len(pid) > pidW {
			pidW = len(pid)
		}
	}

	var b strings.Builder
	header := fmt.Sprintf("%s%-*s  %-*s  %-*s  %s\n",
		Indent,
		nameW, "NAME",
		statusW, "STATUS",
		pidW, "PID",
		"WORKTREE",
	)
	b.WriteString(styles.Muted.Render(header))

	for _, s := range services {
		status := styleServiceStatus(s.Status)
		pid := ""
		if s.PID != 0 {
			pid = strconv.Itoa(s.PID)
		}
		line := fmt.Sprintf("%s%-*s  %-*s  %-*s  %s\n",
			Indent,
			nameW, s.Name,
			statusW+ansiOverhead(status), status,
			pidW, pid,
			styles.Muted.Render(s.WorkDir),
		)
		b.WriteString(line)
	}

	return b.String()
}

func styleServiceStatus(status domain.ServiceStatus) string {
	switch status {
	case domain.ServiceStatusRunning:
		return styles.Success.Render(string(status))
	case domain.ServiceStatusCrashed:
		return styles.Warning.Render(string(status))
	default:
		return styles.Muted.Render(string(status))
	}
}
