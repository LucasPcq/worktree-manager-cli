package newwt

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/tui/components"
)

func TestCreateStepsGating(t *testing.T) {
	params := WizardParams{
		IncludeBranch: true,
		SourceUpdate:  func(string) SourceUpdatePrompt { return SourceUpdatePrompt{} },
	}

	// Disabled → every embedded create step auto-skips.
	off := CreateSteps(params, func([]components.Step) bool { return false })
	if len(off.Steps) == 0 {
		t.Fatal("expected create steps")
	}
	wiz := components.NewWizard(off.Steps)
	for _, s := range off.Steps {
		if s.AutoSkip == nil || !s.AutoSkip(wiz) {
			t.Errorf("step %q should auto-skip when the gate is disabled", s.Name)
		}
	}

	// Enabled → the (otherwise ungated) branch step must not skip.
	on := CreateSteps(params, func([]components.Step) bool { return true })
	onWiz := components.NewWizard(on.Steps)
	for _, s := range on.Steps {
		if s.Name == stepBranchName {
			if s.AutoSkip == nil || s.AutoSkip(onWiz) {
				t.Errorf("branch step should not skip when the gate is enabled")
			}
		}
	}

	// No gate → the branch step carries no AutoSkip at all.
	plain := CreateSteps(params, nil)
	for _, s := range plain.Steps {
		if s.Name == stepBranchName && s.AutoSkip != nil {
			t.Errorf("ungated branch step should have no AutoSkip")
		}
	}
}

func TestCreateStepsSourceRefreshWiring(t *testing.T) {
	// A picked source (Source == "") wires the background branch refresh.
	picked := CreateSteps(WizardParams{IncludeBranch: true}, nil)
	if picked.InitCmd == nil || picked.OnMsg == nil {
		t.Error("expected branch-refresh wiring when the source is picked")
	}
	// A fixed source (--from) needs no picker, hence no refresh.
	fixed := CreateSteps(WizardParams{IncludeBranch: true, Source: "main"}, nil)
	if fixed.InitCmd != nil || fixed.OnMsg != nil {
		t.Error("expected no branch-refresh wiring when the source is fixed")
	}
}
