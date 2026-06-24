// Package checkout builds the interactive multi-step wizard for `wtm checkout`
// (pull request → parent branch → env strategy) with breadcrumb and back nav.
package checkout

import (
	"fmt"
	"sort"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
	"github.com/LucasPcq/wtm/internal/tui/worktreepicker"
)

const (
	stepPR     = "Pull request"
	stepParent = "Parent branch"
	stepEnv    = "Env strategy"
)

// WizardParams holds inputs for the checkout wizard.
type WizardParams struct {
	// PRLoader fetches open PRs asynchronously. When set (no Preselected), the
	// wizard renders instantly and streams the list in with a loading callout.
	PRLoader worktreepicker.PRLoaderFunc
	// Preselected skips the PR-selection step (a number was given as argument).
	Preselected *domain.PRInfo
	// WorktreeBranches lists branches already checked out — their PRs are disabled.
	WorktreeBranches []string
	// LocalBranches are the parent-branch options.
	LocalBranches  []string
	ConfigStrategy domain.EnvStrategy
	// IncludeParent / IncludeEnv add the corresponding step. Set to false when the
	// value is already fixed by a flag (--from / --env-from).
	IncludeParent bool
	IncludeEnv    bool
}

// WizardResult holds the answers from the checkout wizard. FromBranch and
// EnvFromOverride are empty when their step was not included.
type WizardResult struct {
	PR              domain.PRInfo
	FromBranch      string
	EnvFromOverride string
}

// RunWizard displays the interactive checkout wizard. Returns ErrUserAborted on
// Esc at the first step.
func RunWizard(params WizardParams) (WizardResult, error) {
	if params.Preselected == nil {
		return runPicker(params)
	}
	return runPreselected(params)
}

// runPreselected runs the sync parent+env wizard for a known PR.
func runPreselected(params WizardParams) (WizardResult, error) {
	var steps []components.Step
	if params.IncludeParent {
		steps = append(steps, parentStep(params.LocalBranches, params.Preselected.BaseBranch))
	}
	if params.IncludeEnv {
		steps = append(steps, envStep(params.ConfigStrategy))
	}

	if len(steps) == 0 {
		return WizardResult{PR: *params.Preselected}, nil
	}

	final, err := runProgram(components.NewWizard(steps))
	if err != nil {
		return WizardResult{}, err
	}

	res := extractStepValues(final.Steps())
	res.PR = *params.Preselected
	return res, nil
}

// runPicker runs the async wizard: a PR-selection step that streams PRs in,
// followed by the parent (rebuilt from the chosen PR) and env steps.
func runPicker(params WizardParams) (WizardResult, error) {
	var loadedPRs []domain.PRInfo

	steps := []components.Step{prStep(params)}
	if params.IncludeParent {
		steps = append(steps, components.Step{
			Name:    stepParent,
			Summary: selectValueSummary,
			Model:   components.NewSelectList(components.NewSelectListParams{Title: stepParent}),
			Build: func(prev []components.Step) any {
				base := baseBranchForSelection(prev, loadedPRs)
				return parentList(params.LocalBranches, base)
			},
		})
	}
	if params.IncludeEnv {
		steps = append(steps, envStep(params.ConfigStrategy))
	}

	wiz := components.NewWizardWithParams(components.WizardParams{
		Steps:       steps,
		InitCmd:     worktreepicker.PRLoadCmd(params.PRLoader),
		Loading:     true,
		LoadingText: worktreepicker.LoadingPRsText,
		OnMsg: func(w *components.WizardModel, msg tea.Msg) (tea.Cmd, bool) {
			loaded, ok := msg.(worktreepicker.PRsLoadedMsg)
			if !ok {
				return nil, false
			}
			loadedPRs = loaded.PRs
			items := buildPRItems(loaded.PRs, params.WorktreeBranches)
			w.UpdateStepModel(0, func(model any) any {
				sl, ok := model.(components.SelectListModel)
				if !ok {
					return model
				}
				sl.SetItems(items)
				return sl
			})
			w.SetLoading(false)
			w.SetBanner(pickerBanner(loaded))
			return nil, true
		},
	})

	final, err := runProgram(wiz)
	if err != nil {
		return WizardResult{}, err
	}

	res := extractStepValues(final.Steps())
	if sl, ok := stepModel(final.Steps(), stepPR); ok {
		num, _ := strconv.Atoi(sl.Value())
		if pr, found := findPR(loadedPRs, num); found {
			res.PR = pr
		}
	}
	return res, nil
}

func runProgram(wiz components.WizardModel) (components.WizardModel, error) {
	finalModel, err := tea.NewProgram(wiz).Run()
	if err != nil {
		return components.WizardModel{}, fmt.Errorf("wizard: %w", err)
	}
	final, ok := finalModel.(components.WizardModel)
	if !ok || final.Aborted() {
		return components.WizardModel{}, domain.ErrUserAborted
	}
	return final, nil
}

func extractStepValues(steps []components.Step) WizardResult {
	r := WizardResult{}
	if sl, ok := stepModel(steps, stepParent); ok {
		r.FromBranch = sl.Value()
	}
	if sl, ok := stepModel(steps, stepEnv); ok {
		r.EnvFromOverride = sl.Value()
	}
	return r
}

func stepModel(steps []components.Step, name string) (components.SelectListModel, bool) {
	for _, s := range steps {
		if s.Name != name {
			continue
		}
		sl, ok := s.Model.(components.SelectListModel)
		return sl, ok
	}
	return components.SelectListModel{}, false
}

// pickerBanner returns the status box shown once the PR fetch completes: the
// GitHub-connection hint, or a "no PRs" notice when connected but empty.
func pickerBanner(loaded worktreepicker.PRsLoadedMsg) components.WizardBanner {
	if banner := worktreepicker.GHBanner(loaded.Conn); banner.Title != "" {
		return banner
	}
	if len(loaded.PRs) == 0 {
		return components.WizardBanner{Title: "No open pull requests"}
	}
	return components.WizardBanner{}
}

func prStep(params WizardParams) components.Step {
	return components.Step{
		Name: stepPR,
		Model: components.NewSelectList(components.NewSelectListParams{
			Title:       "Select a pull request to checkout",
			Description: "Linked PRs are disabled — use `wtm go <branch>` to enter them",
		}),
		Summary: func(m any) string {
			sl, ok := m.(components.SelectListModel)
			if !ok || sl.Value() == "" {
				return ""
			}
			return "#" + sl.Value()
		},
	}
}

func parentStep(localBranches []string, base string) components.Step {
	return components.Step{
		Name:    stepParent,
		Summary: selectValueSummary,
		Model:   parentList(localBranches, base),
	}
}

func parentList(localBranches []string, base string) components.SelectListModel {
	return components.NewSelectList(components.NewSelectListParams{
		Title:       stepParent,
		Description: "Branch this PR is rebased onto by `wtm sync` (defaults to the PR base)",
		Items:       buildBranchItems(localBranches, base),
	})
}

func baseBranchForSelection(prev []components.Step, prs []domain.PRInfo) string {
	sl, ok := stepModel(prev, stepPR)
	if !ok {
		return ""
	}
	num, _ := strconv.Atoi(sl.Value())
	if pr, found := findPR(prs, num); found {
		return pr.BaseBranch
	}
	return ""
}

func envStep(strategy domain.EnvStrategy) components.Step {
	return components.Step{
		Name: stepEnv,
		Model: components.NewSelectList(components.NewSelectListParams{
			Title:       stepEnv,
			Description: "How to provision .env files in the new worktree",
			Items:       buildEnvItems(strategy),
		}),
		Summary: func(m any) string {
			sl, ok := m.(components.SelectListModel)
			if !ok {
				return ""
			}
			if sl.Value() == "" {
				return "config default"
			}
			return sl.Value()
		},
	}
}

func selectValueSummary(m any) string {
	sl, ok := m.(components.SelectListModel)
	if !ok {
		return ""
	}
	return sl.Value()
}

func findPR(prs []domain.PRInfo, number int) (domain.PRInfo, bool) {
	for _, p := range prs {
		if p.Number == number {
			return p, true
		}
	}
	return domain.PRInfo{}, false
}

// buildPRItems builds the picker items, disabling PRs whose branch already has a
// local worktree (badge "linked") or that come from a fork (badge "fork").
func buildPRItems(prs []domain.PRInfo, worktreeBranches []string) []components.SelectItem {
	linked := make(map[string]bool, len(worktreeBranches))
	for _, b := range worktreeBranches {
		linked[b] = true
	}

	items := make([]components.SelectItem, 0, len(prs))
	for _, p := range prs {
		item := components.SelectItem{
			Label: fmt.Sprintf("#%-4d  %-40s  %s", p.Number, truncate(p.Title, 40), p.Author),
			Value: strconv.Itoa(p.Number),
		}
		switch {
		case linked[p.Branch]:
			item.Disabled = true
			item.Badges = []components.Badge{{Text: "linked", Variant: components.BadgeNeutral}}
		case p.IsFork:
			item.Disabled = true
			item.Badges = []components.Badge{{Text: "fork", Variant: components.BadgeWarning}}
		}
		items = append(items, item)
	}
	return items
}

// buildBranchItems lists local branches with the PR base branch pinned first as
// the pre-selected default.
func buildBranchItems(branches []string, base string) []components.SelectItem {
	items := make([]components.SelectItem, 0, len(branches)+1)
	items = append(items, components.SelectItem{Label: base + " (base)", Value: base})

	rest := make([]string, 0, len(branches))
	for _, b := range branches {
		if b == base {
			continue
		}
		rest = append(rest, b)
	}
	sort.Strings(rest)

	for _, b := range rest {
		items = append(items, components.SelectItem{Label: b, Value: b})
	}
	return items
}

func buildEnvItems(strategy domain.EnvStrategy) []components.SelectItem {
	return []components.SelectItem{
		{Label: "Use config default (" + string(strategy) + ")", Value: ""},
		{Label: "example — copy .env.example → .env", Value: string(domain.EnvStrategyExample)},
		{Label: "main — copy .env from main worktree", Value: string(domain.EnvStrategyMain)},
		{Label: "parent — copy .env from source worktree", Value: string(domain.EnvStrategyParent)},
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}
