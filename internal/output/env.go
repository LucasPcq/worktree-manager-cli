package output

import (
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

// PrintEnvReport renders the per-file drift report of `wtm env`: one block per
// configured env file (its value source and the keys added / missing / in
// conflict / orphaned), then a trailing summary. It emits a raw body with no outer
// blank lines; the caller's frame owns the outer vertical padding. The blank
// between file blocks and before the summary is a genuine separator.
func PrintEnvReport(w io.Writer, result domain.EnvSyncResult) {
	for i, f := range result.Files {
		if i > 0 {
			Blank(w)
		}
		printEnvFile(w, f, result.Check)
	}
	Blank(w)
	printEnvSummary(w, result)
}

// printEnvFile renders one file block. In --check mode adds are described as
// "would add"; otherwise an applied add reads as done.
func printEnvFile(w io.Writer, f domain.EnvFileResult, check bool) {
	SectionTitle(w, fmt.Sprintf("%s   %s",
		f.Target,
		styles.Muted.Render(fmt.Sprintf("strategy: %s  ·  source: %s", f.Strategy, f.Source))))

	added := resolvedAdds(f.Diff)
	conflicts := f.Diff.ByStatus(domain.EnvKeyConflict)
	missing := f.Diff.ByStatus(domain.EnvKeyMissing)
	orphans := f.Diff.ByStatus(domain.EnvKeyOrphan)

	if len(added)+len(conflicts)+len(missing)+len(orphans) == 0 {
		Success(w, styles.Muted.Render("in sync — nothing to reconcile"))
		return
	}

	for _, e := range added {
		from := envSourceName(e.Source, f.ParentBranch)
		switch {
		case check:
			Message(w, fmt.Sprintf("%s %s  %s", styles.Success.Render("+"), e.Key,
				styles.Muted.Render(fmt.Sprintf("would add from %s", from))))
		case f.Applied:
			Success(w, fmt.Sprintf("%s  %s", e.Key, styles.Muted.Render(fmt.Sprintf("added from %s", from))))
		default:
			Message(w, fmt.Sprintf("%s %s  %s", styles.Success.Render("+"), e.Key,
				styles.Muted.Render(fmt.Sprintf("from %s", from))))
		}
	}
	for _, e := range conflicts {
		Warning(w, fmt.Sprintf("%s  conflict — local %s vs %s %s",
			e.Key, quote(e.CurrentValue), envSourceName(e.Source, f.ParentBranch), quote(e.ResolvedValue)))
	}
	for _, e := range missing {
		Warning(w, fmt.Sprintf("%s  missing — needs a value %s",
			e.Key, styles.Muted.Render(fmt.Sprintf("(placeholder %s)", quote(e.Placeholder)))))
	}
	for _, e := range orphans {
		Message(w, fmt.Sprintf("%s %s  %s", styles.Muted.Render("−"), e.Key,
			styles.Muted.Render("orphan — not in any source")))
	}
}

// printEnvSummary prints the trailing one-line verdict.
func printEnvSummary(w io.Writer, result domain.EnvSyncResult) {
	if result.Check {
		if result.HasDrift() {
			Message(w, styles.Muted.Render("Read-only check — run `wtm env` to reconcile."))
			return
		}
		Success(w, "No drift.")
		return
	}

	applied := 0
	for _, f := range result.Files {
		if f.Applied {
			applied++
		}
	}
	if applied == 0 {
		Message(w, styles.Muted.Render("No changes written."))
		return
	}
	Success(w, fmt.Sprintf("Reconciled %d file(s).", applied))
}

// resolvedAdds returns the resolved entries that are additions (absent from the
// child but backed by a real source value), the only resolved keys worth showing.
func resolvedAdds(d domain.EnvDiff) []domain.EnvKeyDiff {
	out := make([]domain.EnvKeyDiff, 0)
	for _, e := range d.ByStatus(domain.EnvKeyResolved) {
		if e.CurrentValue == "" && e.ResolvedValue != "" {
			out = append(out, e)
		}
	}
	return out
}

// envSourceName names the per-key cascade level: the actual parent branch when the
// value came from the parent worktree, else the level itself ("parent" / "main").
func envSourceName(level, parentBranch string) string {
	if level == domain.EnvSourceParent && parentBranch != "" {
		return parentBranch
	}
	return level
}

// quote renders a value for the report, using «empty» for the empty string so a
// blank value is visible rather than invisible.
func quote(v string) string {
	if v == "" {
		return styles.Muted.Render("(empty)")
	}
	return fmt.Sprintf("%q", v)
}

// WriteEnvJSON writes the reconciliation result as pretty-printed JSON. The
// domain result carries its own json tags; it is never framed.
func WriteEnvJSON(w io.Writer, result domain.EnvSyncResult) error {
	return encodeJSON(w, result)
}
