// Package extract renders the interactive selection flow for `wtm extract`:
// a single wizard whose steps are the file multi-select and the target worktree
// picker, so the breadcrumb, step summaries, and back navigation all work.
package extract

import (
	"errors"
	"fmt"
	"strings"

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
	NeedMode     bool
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
	Keep   bool
}

const (
	modeMove = "move"
	modeKeep = "keep"
)

// Run shows the extraction wizard and returns the selected files and target.
// Returns domain.ErrUserAborted when the user cancels at the first step.
func Run(params RunParams) (RunResult, error) {
	var (
		steps         []components.Step
		fileStepIdx   = -1
		targetStepIdx = -1
		modeStepIdx   = -1
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
	if params.NeedMode {
		modeStepIdx = len(steps)
		steps = append(steps, components.Step{
			Name:    "Mode",
			Model:   newModeSelect(params.SourceBranch),
			Summary: modeSummary,
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
	if modeStepIdx >= 0 {
		sl, ok := final.Steps()[modeStepIdx].Model.(components.SelectListModel)
		if !ok {
			return RunResult{}, fmt.Errorf("unexpected model type for mode step")
		}
		result.Keep = sl.Value() == modeKeep
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

func newModeSelect(sourceBranch string) components.SelectListModel {
	return components.NewSelectList(components.NewSelectListParams{
		Title:       "Mode",
		Description: "Move removes the files from the source; copy keeps them.",
		Items: []components.SelectItem{
			{Label: "Move — remove the files from " + sourceBranch, Value: modeMove},
			{Label: "Copy — keep the files in " + sourceBranch, Value: modeKeep},
		},
	})
}

// ConfirmResolve asks whether to apply conflict markers into the target, making
// the consequences explicit. Returns false on No/Esc.
func ConfirmResolve(files []string, targetBranch string) (bool, error) {
	desc := fmt.Sprintf(
		"%s already modified in %q.\n\n"+
			"Applying writes conflict markers there to resolve.\n"+
			"Nothing is removed from the source.\n"+
			"Resolve in %q then discard there, or discard in %q to undo.",
		joinFiles(files), targetBranch, targetBranch, targetBranch)

	cm := components.NewConfirm(components.NewConfirmParams{
		Title:       "Apply conflict markers in " + targetBranch + "?",
		Description: desc,
	})
	confirmed, err := components.RunStandaloneConfirm(cm)
	if errors.Is(err, components.ErrAborted) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return confirmed, nil
}

func joinFiles(files []string) string {
	if len(files) == 1 {
		return files[0] + " was"
	}
	return strings.Join(files, ", ") + " were"
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

func modeSummary(model any) string {
	sl, ok := model.(components.SelectListModel)
	if !ok {
		return ""
	}
	if sl.Value() == modeKeep {
		return "copy"
	}
	return "move"
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
