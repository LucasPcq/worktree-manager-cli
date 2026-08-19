package dashboard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	pruneflow "github.com/LucasPcq/wtm/internal/flow/prune"
)

// collect drains what a presenter posted, without a running model behind it.
func collect(t *testing.T, run func(send func(tea.Msg))) []string {
	t.Helper()
	msgs := make(chan tea.Msg, 32)
	run(func(msg tea.Msg) { msgs <- msg })
	close(msgs)

	var lines []string
	for msg := range msgs {
		if line, ok := msg.(OutputLineMsg); ok {
			lines = append(lines, line.Text)
		}
	}
	return lines
}

// A worktree the user checked and the safety re-gate then dropped must say so.
// Without it the run silently does less than what was confirmed, and the panel
// gives the user no way to notice.
func TestPrunePresenterReportsWhatWasSkipped(t *testing.T) {
	outcome := pruneflow.Outcome{Result: domain.PruneResult{
		Pruned:  []domain.PruneCandidate{{Branch: "merged-wt", Reason: domain.PruneReasonPRMerged}},
		Skipped: []domain.PruneSkip{{Branch: "dirty-wt", Reason: domain.PruneSkipDirty}},
	}}

	lines := collect(t, func(send func(tea.Msg)) {
		if err := (prunePresenter{presenter{send: send}}).Pruned(outcome); err != nil {
			t.Fatalf("Pruned: %v", err)
		}
	})

	body := strings.Join(lines, "\n")
	if !strings.Contains(body, "merged-wt") {
		t.Errorf("the removed worktree must be reported:\n%s", body)
	}
	if !strings.Contains(body, "dirty-wt") || !strings.Contains(body, domain.PruneLabelDirty) {
		t.Errorf("the skipped worktree must be reported with its reason:\n%s", body)
	}
}

func TestPrunePresenterSaysWhenNothingMatched(t *testing.T) {
	lines := collect(t, func(send func(tea.Msg)) {
		if err := (prunePresenter{presenter{send: send}}).Pruned(pruneflow.Outcome{Empty: true}); err != nil {
			t.Fatalf("Pruned: %v", err)
		}
	})

	if len(lines) != 1 || lines[0] != domain.PruneNothingToPrune {
		t.Errorf("lines = %v, want the empty run to say so once", lines)
	}
}
