package output

import (
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type ComposePortsReportParams struct {
	// Patched are the rewrites that were applied, empty when none were.
	Patched map[string][]domain.ComposePortBinding
	// Written is what each job gained in run.toml.
	Written map[string]map[string]int
	// Withheld are the ports wtm detected but did not declare, and JobsByFile
	// names the job each one's file backs so the fix can be a command to paste.
	Withheld   []domain.ComposePortBinding
	JobsByFile map[string]string
	// Dropped are the detected declarations withdrawn to keep run.toml loadable.
	Dropped []rules.DroppedPort
	// Unreadable are the compose files the scan could not open or parse.
	Unreadable []domain.ComposeScan
	// Changed are the files that moved between the scan and the write, mapped to
	// the reason, and Orphaned those whose ports found no job to carry them.
	Changed  map[string]string
	Orphaned []string
}

// ComposePortsReport prints what the detection did and, just as importantly,
// what it declined to do. It emits a raw body with no surrounding blank lines;
// the caller's frame owns the padding. Nothing to say prints nothing.
func ComposePortsReport(w io.Writer, params ComposePortsReportParams) {
	if len(params.Patched) > 0 {
		Blank(w)
		Callout(w, domain.ComposePatchedTitle, rules.ComposePatchLines(params.Patched))
	}

	if len(params.Written) > 0 {
		Blank(w)
		Callout(w, domain.ComposePortsTitle, rules.ComposePortsWrittenLines(params.Written))
	}

	if len(params.Withheld) > 0 {
		Blank(w)
		Callout(w, domain.ComposeWithheldTitle, withheldLines(params))
	}

	if len(params.Dropped) > 0 {
		Blank(w)
		lines := make([]string, 0, len(params.Dropped))
		for _, d := range params.Dropped {
			lines = append(lines, rules.ComposeDroppedLine(d))
		}
		Callout(w, domain.ComposeDroppedTitle, lines)
	}

	if len(params.Changed) > 0 {
		Blank(w)
		lines := make([]string, 0, len(params.Changed))
		for _, file := range rules.SortedComposeFiles(params.Changed) {
			lines = append(lines, fmt.Sprintf(domain.ComposeUnreadableFmt, file, params.Changed[file]))
		}
		Callout(w, domain.ComposeChangedTitle, lines)
	}

	if len(params.Orphaned) > 0 {
		Blank(w)
		lines := make([]string, 0, len(params.Orphaned))
		for _, file := range params.Orphaned {
			lines = append(lines, fmt.Sprintf(domain.ComposeOrphanFmt, file))
		}
		Callout(w, domain.ComposeOrphanTitle, lines)
	}

	for _, scan := range params.Unreadable {
		Blank(w)
		Warning(w, rules.ComposeUnreadableLine(scan))
	}
}

func withheldLines(params ComposePortsReportParams) []string {
	var lines []string
	for _, b := range params.Withheld {
		lines = append(lines, rules.ComposeWithheldLine(b))
		for _, fix := range rules.ComposeFixLines(rules.ComposeFixLinesParams{Binding: b, Job: params.JobsByFile[b.File]}) {
			lines = append(lines, fmt.Sprintf(domain.ComposeFixIndentFmt, fix))
		}
	}
	return lines
}
