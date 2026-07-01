// Package branchrefresh wires the "r" refresh key into branch pickers: it
// re-fetches origin, recomputes the divergence-tagged candidates, and rebuilds
// the current wizard step in place. Each branch step must read its candidates
// from the shared holder (so a rebuild picks up the fresh list) and set
// Step.CanRefresh so the key is gated to branch steps only.
package branchrefresh

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/branch"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// RefreshedMsg carries the re-fetched branch candidates back into the wizard.
type RefreshedMsg struct {
	Candidates []domain.BranchCandidate
}

// Cmd fetches origin and recomputes the branch candidates off the UI thread.
func Cmd(projectDir string) tea.Cmd {
	return func() tea.Msg {
		return RefreshedMsg{Candidates: branch.Refresh(branch.ListParams{ProjectDir: projectDir})}
	}
}

// HandleParams holds inputs for Handle.
type HandleParams struct {
	Wizard     *components.WizardModel
	Msg        tea.Msg
	ProjectDir string
	// Holder is the candidate slice the branch steps' Build hooks read from; Handle
	// overwrites it with the fresh candidates before rebuilding the current step.
	Holder *[]domain.BranchCandidate
}

// Handler returns a wizard message handler that wires the refresh key/message for
// a picker whose branch steps read from holder. Pickers that also handle async
// messages chain it first — it returns handled=false for messages it does not own.
func Handler(projectDir string, holder *[]domain.BranchCandidate) components.WizardMsgHandler {
	return func(w *components.WizardModel, msg tea.Msg) (tea.Cmd, bool) {
		return Handle(HandleParams{
			Wizard:     w,
			Msg:        msg,
			ProjectDir: projectDir,
			Holder:     holder,
		})
	}
}

// Handle processes the refresh key and the RefreshedMsg for a branch picker.
// Call it first from a picker's OnMsg: it returns handled=false for messages it
// does not own (the refresh key on a non-branch/filtering step, or any unrelated
// message) so the picker can chain its own handling.
func Handle(params HandleParams) (tea.Cmd, bool) {
	w := params.Wizard

	if key, ok := params.Msg.(tea.KeyMsg); ok && key.String() == domain.KeyRefresh {
		if !w.CurrentStepCanRefresh() || w.Loading() {
			return nil, false
		}
		sl, ok := w.CurrentStepModel().(components.SelectListModel)
		if !ok || sl.Filtering() {
			return nil, false
		}
		return tea.Batch(w.StartLoading(domain.LoadingBranchesText), Cmd(params.ProjectDir)), true
	}

	if msg, ok := params.Msg.(RefreshedMsg); ok {
		*params.Holder = msg.Candidates
		w.RebuildCurrentStep()
		w.SetLoading(false)
		return nil, true
	}

	return nil, false
}
