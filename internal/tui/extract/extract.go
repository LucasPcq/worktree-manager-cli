// Package extract renders the interactive selection flow for `wtm wt extract`:
// a single wizard whose steps are the file multi-select and the target worktree
// picker, so the breadcrumb, step summaries, and back navigation all work.
package extract

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// newWorktreeValue is the sentinel SelectList value for the "create new" entry.
const newWorktreeValue = "\x00new-worktree"

// TargetChoice is the outcome of the target selection.
type TargetChoice struct {
	CreateNew bool
	Branch    string
}

// RunParams holds inputs for the interactive extraction wizard. NeedFiles and
// NeedTarget toggle the corresponding steps, letting the command skip a step
// already resolved from a flag.
type RunParams struct {
	Files        []domain.ExtractFile
	Worktrees    []domain.WorktreeStatus
	SourceBranch string
	NeedFiles    bool
	NeedTarget   bool
	// Preselected marks files already chosen on a previous pass so they stay
	// checked when the wizard is re-entered.
	Preselected []string
	// StartAtTarget re-enters the wizard on the Target step (with Files already
	// answered), used when the user backs out of the new-worktree sub-flow.
	StartAtTarget bool
}

// RunResult holds the wizard answers. Only the fields for the requested steps
// are populated.
type RunResult struct {
	Files  []string
	Target TargetChoice
}

// Run shows the extraction wizard and returns the selected files and target.
// Returns domain.ErrUserAborted when the user cancels at the first step.
func Run(params RunParams) (RunResult, error) {
	var (
		steps         []components.Step
		fileStepIdx   = -1
		targetStepIdx = -1
	)

	if params.NeedFiles {
		fileStepIdx = len(steps)
		steps = append(steps, components.Step{
			Name:    "Files",
			Model:   newFileSelect(params.Files, params.Preselected),
			Summary: fileSummary,
		})
	}
	if params.NeedTarget {
		targetStepIdx = len(steps)
		steps = append(steps, components.Step{
			Name:    "Target worktree",
			Model:   newTargetSelect(params.Worktrees, params.SourceBranch),
			Summary: targetSummary,
		})
	}

	start := 0
	if params.StartAtTarget && targetStepIdx > 0 {
		start = targetStepIdx
	}
	wiz := components.NewWizardAtStep(steps, start)
	finalModel, err := tea.NewProgram(wiz).Run()
	if err != nil {
		return RunResult{}, fmt.Errorf("extract wizard: %w", err)
	}

	final, ok := finalModel.(components.WizardModel)
	if !ok || final.Aborted() {
		return RunResult{}, domain.ErrUserAborted
	}

	var result RunResult
	if fileStepIdx >= 0 {
		ms, ok := final.Steps()[fileStepIdx].Model.(components.MultiSelectModel)
		if !ok {
			return RunResult{}, fmt.Errorf("unexpected model type for files step")
		}
		result.Files = ms.Values()
	}
	if targetStepIdx >= 0 {
		sl, ok := final.Steps()[targetStepIdx].Model.(components.SelectListModel)
		if !ok {
			return RunResult{}, fmt.Errorf("unexpected model type for target step")
		}
		result.Target = parseTarget(sl.Value())
	}
	return result, nil
}

func newFileSelect(files []domain.ExtractFile, preselected []string) components.MultiSelectModel {
	chosen := make(map[string]struct{}, len(preselected))
	for _, p := range preselected {
		chosen[p] = struct{}{}
	}

	items := make([]components.MultiSelectItem, 0, len(files))
	for _, f := range files {
		_, isChosen := chosen[f.Path]
		items = append(items, components.MultiSelectItem{
			Label:    f.Path,
			Value:    f.Path,
			Tag:      rules.ExtractStatusLabel(f.Status),
			Variant:  tagVariant(f.Status),
			Selected: isChosen,
		})
	}

	return components.NewMultiSelect(components.NewMultiSelectParams{
		Title:       "Select files to extract",
		Description: "Space to toggle, enter to confirm.",
		Items:       items,
		Validate: func(vals []string) error {
			if len(vals) == 0 {
				return fmt.Errorf("select at least one file")
			}
			return nil
		},
	})
}

func newTargetSelect(worktrees []domain.WorktreeStatus, sourceBranch string) components.SelectListModel {
	items := []components.SelectItem{
		{Label: "✚ Create a new worktree…", Value: newWorktreeValue},
	}

	existing := make([]components.SelectItem, 0, len(worktrees))
	for _, w := range worktrees {
		if w.Branch == sourceBranch {
			continue
		}
		existing = append(existing, components.SelectItem{Label: w.Branch, Value: w.Branch})
	}
	if len(existing) > 0 {
		items = append(items, components.SelectItem{Separator: true})
		items = append(items, existing...)
	}

	return components.NewSelectList(components.NewSelectListParams{
		Title:       "Target worktree",
		Description: "Where to move the selected files",
		Items:       items,
	})
}

func fileSummary(model any) string {
	ms, ok := model.(components.MultiSelectModel)
	if !ok {
		return ""
	}
	n := len(ms.Values())
	if n == 1 {
		return "1 file"
	}
	return fmt.Sprintf("%d files", n)
}

func targetSummary(model any) string {
	sl, ok := model.(components.SelectListModel)
	if !ok {
		return ""
	}
	if sl.Value() == newWorktreeValue {
		return "new worktree"
	}
	return sl.Value()
}

func parseTarget(value string) TargetChoice {
	if value == newWorktreeValue {
		return TargetChoice{CreateNew: true}
	}
	return TargetChoice{Branch: value}
}

// tagVariant maps a file's extraction status to its tag color: yellow for a
// modification, green for a new file, red for a deletion.
func tagVariant(status domain.ExtractFileStatus) components.TagVariant {
	switch status {
	case domain.ExtractStatusUntracked:
		return components.TagSuccess
	case domain.ExtractStatusDeleted:
		return components.TagDanger
	default:
		return components.TagWarning
	}
}
