package output

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

// relTarget renders a worktree's target path relative to the repo (base_path +
// directory name) so the preview stays readable instead of showing absolute paths.
func relTarget(basePath, toPath string) string {
	return filepath.Join(basePath, filepath.Base(toPath))
}

// FormatRelocatePlan prints the planned actions grouped by category (to apply,
// skipped, blocked) with blank lines between groups, then a note when the user
// will be asked to choose adoption parents.
func FormatRelocatePlan(w io.Writer, plan domain.RelocatePlan) {
	var apply, skipped, blocked []domain.RelocateStep
	adoptions, noops := 0, 0
	for _, step := range plan.Steps {
		switch step.Status {
		case domain.RelocateStatusMove, domain.RelocateStatusAdopt:
			apply = append(apply, step)
			if step.Adopt {
				adoptions++
			}
		case domain.RelocateStatusSkippedDirty, domain.RelocateStatusSkippedLocked:
			skipped = append(skipped, step)
		case domain.RelocateStatusBlockedDest:
			blocked = append(blocked, step)
		case domain.RelocateStatusNoop:
			noops++
		}
	}

	Blank(w)
	fmt.Fprintln(w, styles.RenderIntro(styles.IntroParams{
		Width: 80,
		Title: "Relocate worktrees",
		Body: fmt.Sprintf(
			"Aligns every worktree with base_path %q: scattered worktrees are moved there, and "+
				"worktrees created outside wtm are adopted (their parent recorded so `wtm sync` can use them).\n"+
				"Dirty or locked worktrees are skipped — re-run with --force to move them. "+
				"A target path that is already occupied is never overwritten.",
			plan.BasePath),
	}))
	Blank(w)

	// Sections are separated by a blank line but none trails the last one, so the
	// caller controls the spacing to whatever follows (the wizard, or "Dry run").
	section := newSectionWriter(w)

	if len(apply) > 0 {
		section(func() {
			SectionTitle(w, fmt.Sprintf("To apply (%d)", len(apply)))
			for _, step := range apply {
				Message(w, styles.Muted.Render("• ")+planApplyLine(plan.BasePath, step))
			}
		})
	}
	if len(skipped) > 0 {
		section(func() {
			SectionTitle(w, fmt.Sprintf("Skipped (%d)", len(skipped)))
			for _, step := range skipped {
				Warning(w, planSkipLine(step))
			}
		})
	}
	if len(blocked) > 0 {
		section(func() {
			SectionTitle(w, fmt.Sprintf("Blocked (%d)", len(blocked)))
			for _, step := range blocked {
				Danger(w, fmt.Sprintf("%s — target path already occupied: %s", step.Branch, step.ToPath))
			}
		})
	}
	if noops > 0 {
		section(func() {
			Message(w, styles.Muted.Render(fmt.Sprintf("%d worktree(s) already in place.", noops)))
		})
	}
	if adoptions > 0 {
		section(func() {
			Message(w, styles.Primary.Render(fmt.Sprintf("→ You'll choose a parent branch for %d worktree(s).", adoptions)))
		})
	}
}

// newSectionWriter returns a function that renders a section, inserting a blank
// line before every section except the first. No blank trails the last section.
func newSectionWriter(w io.Writer) func(render func()) {
	first := true
	return func(render func()) {
		if !first {
			Blank(w)
		}
		first = false
		render()
	}
}

func planApplyLine(basePath string, step domain.RelocateStep) string {
	if step.Status == domain.RelocateStatusAdopt {
		return fmt.Sprintf("%s %s", step.Branch, styles.Muted.Render("(adopt in place)"))
	}
	suffix := ""
	if step.Adopt {
		suffix = styles.Muted.Render(" (+ adopt)")
	}
	return fmt.Sprintf("%s %s %s%s", step.Branch, styles.Muted.Render("→"), relTarget(basePath, step.ToPath), suffix)
}

func planSkipLine(step domain.RelocateStep) string {
	if step.Status == domain.RelocateStatusSkippedLocked {
		return fmt.Sprintf("%s — worktree is locked (use --force)", step.Branch)
	}
	return fmt.Sprintf("%s — uncommitted changes (use --force)", step.Branch)
}

// FormatRelocateResult prints the outcome as a clear conclusion: a headline
// (success or "with issues") with a one-line tally, the detail of what was
// actually applied (with parents), then condensed skipped/blocked lines. It is
// deliberately distinct from FormatRelocatePlan so the end state doesn't read
// like a repeat of the opening preview.
func FormatRelocateResult(w io.Writer, result domain.RelocateResult) {
	var done, errored []domain.RelocateStepResult
	var skipped, blocked []string
	for _, step := range result.Steps {
		switch step.Status {
		case domain.RelocateStatusMoved, domain.RelocateStatusMovedAdopted, domain.RelocateStatusAdopted:
			done = append(done, step)
		case domain.RelocateStatusSkippedDirty, domain.RelocateStatusSkippedLocked:
			skipped = append(skipped, step.Branch)
		case domain.RelocateStatusBlockedDest:
			blocked = append(blocked, step.Branch)
		case domain.RelocateStatusError:
			errored = append(errored, step)
		}
	}

	hasIssue := len(blocked) > 0 || len(errored) > 0
	headline := relocateTally(relocateTallyParams{
		Applied: len(done),
		Skipped: len(skipped),
		Issues:  len(blocked) + len(errored),
	})
	if hasIssue {
		Warning(w, "Relocation finished with issues  "+styles.Muted.Render(headline))
	} else {
		Success(w, "Relocation complete  "+styles.Muted.Render(headline))
	}
	Blank(w)

	for _, step := range done {
		Success(w, resultDoneLine(result.BasePath, step))
	}
	if len(done) > 0 && (len(skipped) > 0 || len(blocked) > 0 || len(errored) > 0) {
		Blank(w)
	}

	if len(skipped) > 0 {
		Warning(w, fmt.Sprintf("Skipped: %s (re-run with --force)", strings.Join(skipped, ", ")))
	}
	if len(blocked) > 0 {
		Danger(w, fmt.Sprintf("Blocked: %s (target path occupied)", strings.Join(blocked, ", ")))
	}
	for _, step := range errored {
		Error(w, fmt.Sprintf("%s failed — %s", step.Branch, step.Detail))
	}

	if result.BasePathUpdated {
		Blank(w)
		Success(w, fmt.Sprintf("config base_path updated to %q", result.BasePath))
	}
	Blank(w)
}

type relocateTallyParams struct {
	Applied int
	Skipped int
	Issues  int
}

// relocateTally renders a compact "N applied · N skipped · N blocked" summary,
// omitting zero counts.
func relocateTally(params relocateTallyParams) string {
	parts := make([]string, 0, 3)
	if params.Applied > 0 {
		parts = append(parts, fmt.Sprintf("%d applied", params.Applied))
	}
	if params.Skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", params.Skipped))
	}
	if params.Issues > 0 {
		parts = append(parts, fmt.Sprintf("%d blocked", params.Issues))
	}
	if len(parts) == 0 {
		return "nothing to do"
	}
	return strings.Join(parts, " · ")
}

func resultDoneLine(basePath string, step domain.RelocateStepResult) string {
	switch step.Status {
	case domain.RelocateStatusAdopted:
		return fmt.Sprintf("%s adopted in place (parent: %s)", step.Branch, step.Parent)
	case domain.RelocateStatusMovedAdopted:
		return fmt.Sprintf("%s %s %s (adopted, parent: %s)", step.Branch, styles.Muted.Render("→"), relTarget(basePath, step.ToPath), step.Parent)
	default:
		return fmt.Sprintf("%s %s %s", step.Branch, styles.Muted.Render("→"), relTarget(basePath, step.ToPath))
	}
}

// WriteRelocateResultJSON writes the relocate result as pretty-printed JSON.
func WriteRelocateResultJSON(w io.Writer, result domain.RelocateResult) error {
	return encodeJSON(w, result)
}
