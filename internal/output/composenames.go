package output

import (
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type ComposeNamesReportParams struct {
	Patched  map[string][]domain.ComposeAbsoluteName
	Withheld []domain.ComposeAbsoluteName
}

// ComposeNamesReport tells what `run init` did with the names the selected
// compose files pin absolutely, and what it left colliding.
func ComposeNamesReport(w io.Writer, params ComposeNamesReportParams) {
	if len(params.Patched) > 0 {
		lines := rules.ComposeNamePatchLines(params.Patched)
		if rules.ComposeNamesRenameAVolume(params.Patched) {
			lines = append(lines, "", domain.ComposeNamesVolumeWarning)
		}
		Blank(w)
		Section(w, domain.ComposeNamesPatchedTitle, lines)
	}

	if len(params.Withheld) > 0 {
		lines := make([]string, 0, len(params.Withheld))
		for _, n := range params.Withheld {
			lines = append(lines, rules.ComposeNameWithheldLine(n))
		}
		Blank(w)
		Callout(w, domain.ComposeNamesWithheldTitle, lines)
	}
}
