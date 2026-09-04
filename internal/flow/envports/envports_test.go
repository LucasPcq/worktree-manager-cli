package envports_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/envports"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
)

// A project with no [[env_port]] link has nothing to settle, so the step must
// stay silent rather than confirm an empty rewrite.
func TestSettleWithoutLinksAsksNothing(t *testing.T) {
	prompter := &flowtest.ScriptedPrompter{Confirmed: true}
	presenter := &flowtest.Recorder{}

	err := envports.Settle(envports.Params{
		Context:      flow.Context{ProjectDir: t.TempDir(), StateDir: t.TempDir()},
		Branch:       "feat/x",
		WorktreePath: t.TempDir(),
		Prompter:     prompter,
		Presenter:    presenter,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prompter.Confirms != 0 {
		t.Errorf("asked %d confirmations, want none", prompter.Confirms)
	}
	if len(presenter.Statuses) != 0 {
		t.Errorf("reported %+v, want nothing", presenter.Statuses)
	}
}
