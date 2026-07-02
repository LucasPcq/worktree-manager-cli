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
	newpicker "github.com/LucasPcq/wtm/internal/tui/newwt"
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
	// Create configures the embedded "create new worktree" sub-flow, inserted after
	// the target step and auto-skipped unless the target is "create new". Only used
	// when NeedTarget. Its Source is "" (source picked), IncludeBranch true,
	// IncludeEnv false (the new worktree uses the config-default env strategy).
	Create newpicker.WizardParams
}

// RunResult holds the wizard answers. Only the fields for the requested steps
// are populated.
type RunResult struct {
	Files  []string
	Target TargetChoice
	Keep   bool
	// Create holds the new-worktree answers (branch/source/fast-forward) when
	// Target.CreateNew; zero otherwise.
	Create newpicker.WizardResult
}

const (
	modeMove = "move"
	modeKeep = "keep"
)

// Step name and action value for the final recap.
const (
	stepConfirm    = "Confirm"
	extractConfirm = "extract"
)

// Step names, used for construction and name-based value extraction.
const (
	stepFiles  = "Files"
	stepTarget = "Target worktree"
	stepMode   = "Mode"
)

// Run shows the extraction wizard and returns the selected files, target, and —
// when the target is "create new" — the new-worktree answers. The create sub-flow
// is embedded so a single combined recap covers both create and extract. Returns
// domain.ErrUserAborted when the user cancels.
func Run(params RunParams) (RunResult, error) {
	var steps []components.Step

	if params.NeedFiles {
		steps = append(steps, components.Step{
			Name:    stepFiles,
			Model:   newFileSelect(params.Files, nil),
			Summary: fileSummary,
		})
	}
	if params.NeedTarget {
		steps = append(steps, components.Step{
			Name:    stepTarget,
			Model:   newTargetSelect(params.Worktrees, params.SourceBranch),
			Summary: targetSummary,
		})
	}

	// The create sub-flow, inserted after the target step and auto-skipped unless
	// the target is "create new" — so creating the worktree and extracting share
	// one combined recap instead of two.
	var createFlow newpicker.CreateFlow
	if params.NeedTarget {
		createFlow = newpicker.CreateSteps(params.Create, isNewTarget)
		steps = append(steps, createFlow.Steps...)
	}

	if params.NeedMode {
		steps = append(steps, components.Step{
			Name:    stepMode,
			Model:   newModeSelect(newModeSelectParams{SourceBranch: params.SourceBranch}),
			Summary: modeSummary,
		})
	}

	// Combined recap: files + target (existing branch, or "new worktree <branch>
	// from <source>") + mode + ⚠ create warnings, then "No, cancel".
	steps = append(steps, components.RecapStep(components.RecapStepParams{
		Name:  stepConfirm,
		Build: func(prev []components.Step) components.RecapContent { return buildCombinedRecap(prev, params) },
	}))

	wp := components.WizardParams{
		Steps:       steps,
		InitCmd:     createFlow.InitCmd,
		OnMsg:       createFlow.OnMsg,
		LoadingText: createFlow.LoadingText,
		Loading:     createFlow.InitCmd != nil,
	}
	finalModel, err := tea.NewProgram(components.NewWizardWithParams(wp)).Run()
	if err != nil {
		return RunResult{}, fmt.Errorf("extract wizard: %w", err)
	}

	final, ok := finalModel.(components.WizardModel)
	if !ok || final.Aborted() {
		return RunResult{}, domain.ErrUserAborted
	}
	steps = final.Steps()
	if v, _ := stepSelectValue(steps, stepConfirm); v == domain.WizardCancelValue {
		return RunResult{}, domain.ErrUserAborted
	}

	var result RunResult
	if ms, ok := stepModelByName(steps, stepFiles).(components.MultiSelectModel); ok {
		result.Files = ms.Values()
	}
	if v, ok := stepSelectValue(steps, stepTarget); ok {
		result.Target = parseTarget(v)
		if result.Target.CreateNew {
			result.Create = newpicker.ReadCreateResult(steps, params.Create)
		}
	}
	if v, ok := stepSelectValue(steps, stepMode); ok {
		result.Keep = v == modeKeep
	}
	return result, nil
}

// isNewTarget reports whether the chosen target is the "create new worktree" entry
// — the gate for the embedded create steps.
func isNewTarget(steps []components.Step) bool {
	v, _ := stepSelectValue(steps, stepTarget)
	return v == newWorktreeValue
}

// buildCombinedRecap recaps whichever of the file/target/mode steps are present,
// expanding a "create new" target into its branch/source and folding in the create
// warnings.
func buildCombinedRecap(prev []components.Step, params RunParams) components.RecapContent {
	var lines []string
	if ms, ok := stepModelByName(prev, stepFiles).(components.MultiSelectModel); ok {
		lines = append(lines, "Files:   "+strings.Join(ms.Values(), ", "))
	}

	action := "extract"
	if sl, ok := stepModelByName(prev, stepTarget).(components.SelectListModel); ok {
		if isNewTarget(prev) {
			cr := newpicker.ReadCreateResult(prev, params.Create)
			source := cr.FromBranch
			if cr.FastForwardSource {
				source += " (fast-forward to origin)"
			}
			lines = append(lines, "Target:  new worktree "+cr.BranchName+" from "+source)
			action = "create & extract"
		} else {
			lines = append(lines, "Target:  "+targetSummary(sl))
		}
	}

	if sl, ok := stepModelByName(prev, stepMode).(components.SelectListModel); ok {
		lines = append(lines, "Mode:    "+modeSummary(sl))
	}

	if isNewTarget(prev) {
		if warnings := newpicker.CreateWarnings(prev, params.Create); len(warnings) > 0 {
			lines = append(lines, "")
			lines = append(lines, warnings...)
		}
	}

	return components.RecapContent{
		Description: strings.Join(lines, "\n"),
		Actions:     []components.SelectItem{{Label: "Yes, " + action, Value: extractConfirm}},
	}
}

// stepSelectValue reads the chosen value of the named SelectList step.
func stepSelectValue(steps []components.Step, name string) (string, bool) {
	if sl, ok := stepModelByName(steps, name).(components.SelectListModel); ok {
		return sl.Value(), true
	}
	return "", false
}

// stepModelByName returns the model of the named step, or nil when absent.
func stepModelByName(steps []components.Step, name string) any {
	for _, s := range steps {
		if s.Name == name {
			return s.Model
		}
	}
	return nil
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
		Description: domain.MultiSelectHint,
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

// newModeSelectParams holds inputs for newModeSelect.
type newModeSelectParams struct {
	SourceBranch string
}

func newModeSelect(params newModeSelectParams) components.SelectListModel {
	return components.NewSelectList(components.NewSelectListParams{
		Title:       "Mode",
		Description: "Move removes the files from the source; copy keeps them.",
		Items: []components.SelectItem{
			{Label: "Move — remove the files from " + params.SourceBranch, Value: modeMove},
			{Label: "Copy — keep the files in " + params.SourceBranch, Value: modeKeep},
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
