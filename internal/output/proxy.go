package output

import (
	"fmt"
	"io"
	"strconv"

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
	Title  string
	Files  []domain.ProxyPlannedFile
	Script string
}

// ProxyPlanReport shows a privileged write in full before it is asked for.
func ProxyPlanReport(w io.Writer, params ProxyPlanReportParams) {
	SectionTitle(w, params.Title)
	for _, file := range params.Files {
		Section(w, file.Path, []string{file.Content})
	}
	if params.Script != "" {
		Section(w, domain.ProxyInstallRecapScript, []string{params.Script})
	}
}
