package output

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

func WriteProxyStatusJSON(w io.Writer, status domain.ProxyStatus) error {
	return encodeJSON(w, status)
}

func ProxyStatusReport(w io.Writer, status domain.ProxyStatus) {
	lines := []string{
		fmt.Sprintf(domain.ProxyStatusBindFmt, status.BindPort),
		fmt.Sprintf(domain.ProxyStatusPublicFmt, strconv.Itoa(status.PublicPort)),
		fmt.Sprintf(domain.ProxyStatusRedirectFmt, redirectState(status)),
	}
	if status.ExampleURL != "" {
		lines = append(lines, fmt.Sprintf(domain.ProxyStatusExampleFmt, status.ExampleURL))
	}
	Section(w, domain.ProxyStatusTitle, lines)

	if status.Diverged {
		Callout(w, domain.ProxyDivergedTitle, []string{domain.ProxyDivergedLine, domain.ProxyDivergedFix})
	}
}

func redirectState(status domain.ProxyStatus) string {
	if !status.Supported {
		return domain.ProxyStatusUnsupported
	}
	if !status.Installed {
		return domain.ProxyStatusNotInstalled
	}
	return fmt.Sprintf(domain.ProxyStatusInstalledFmt, status.Mechanism, status.BindPort)
}

type ProxyPlanReportParams struct {
	Files  []domain.ProxyPlannedFile
	Script string
	// Full prints each file's contents rather than what changes in it.
	Full bool
	// Reversible adds the two lines only an install has to offer.
	Reversible bool
}

// ProxyPlanReport shows what a privileged write touches before it is asked for.
func ProxyPlanReport(w io.Writer, params ProxyPlanReportParams) {
	SectionTitle(w, fmt.Sprintf(domain.ProxyInstallRecapTitleFmt, len(params.Files)))
	Blank(w)

	if params.Full {
		for _, file := range params.Files {
			Section(w, file.Path, strings.Split(strings.TrimRight(file.Content, "\n"), "\n"))
		}
	}
	if !params.Full {
		lines := make([]string, 0, len(params.Files))
		for _, file := range params.Files {
			lines = append(lines, fmt.Sprintf(domain.ProxyPlanFileFmt, file.Path, file.Change))
		}
		for _, line := range lines {
			Message(w, line)
		}
		Blank(w)
	}

	if params.Script != "" {
		Section(w, domain.ProxyInstallRecapScript, strings.Split(params.Script, "\n"))
		Blank(w)
	}
	if params.Reversible {
		Message(w, domain.ProxyInstallRecapReverse)
		if !params.Full {
			Message(w, domain.ProxyInstallRecapFull)
		}
	}
}
