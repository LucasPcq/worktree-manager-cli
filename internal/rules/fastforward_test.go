package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestFastForwardBlockersOnlyReportsDirty(t *testing.T) {
	cases := []struct {
		name  string
		check domain.FastForwardCheck
		want  int
	}{
		{
			name:  "clean and behind has no blocker",
			check: domain.FastForwardCheck{Branch: "feat", HasUpstream: true, Behind: 3, State: domain.DivergenceBehind},
			want:  0,
		},
		{
			name:  "dirty and behind has one",
			check: domain.FastForwardCheck{Branch: "feat", HasUpstream: true, Behind: 3, State: domain.DivergenceBehind, IsDirty: true},
			want:  1,
		},
		{
			name:  "dirty and diverged has none: a refusal is not a blocker",
			check: domain.FastForwardCheck{Branch: "feat", HasUpstream: true, Ahead: 1, Behind: 3, State: domain.DivergenceDiverged, IsDirty: true},
			want:  0,
		},
		{
			name:  "dirty with no upstream has none",
			check: domain.FastForwardCheck{Branch: "feat", IsDirty: true, State: domain.DivergenceUnknown},
			want:  0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := len(FastForwardBlockers(tc.check)); got != tc.want {
				t.Fatalf("blockers = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestFastForwardBlockerCarriesTheDirtyKey(t *testing.T) {
	blockers := FastForwardBlockers(domain.FastForwardCheck{
		Branch: "feat", HasUpstream: true, Behind: 1, State: domain.DivergenceBehind, IsDirty: true,
	})
	if len(blockers) != 1 {
		t.Fatalf("blockers = %d, want 1", len(blockers))
	}
	if blockers[0].Key != domain.FastForwardBlockerDirty {
		t.Fatalf("key = %q, want %q", blockers[0].Key, domain.FastForwardBlockerDirty)
	}
	if blockers[0].Label != domain.FastForwardWarnDirty {
		t.Fatalf("label = %q, want %q", blockers[0].Label, domain.FastForwardWarnDirty)
	}
}

func TestFastForwardRefusalCoversWhatForceMustNeverLift(t *testing.T) {
	cases := []struct {
		name     string
		check    domain.FastForwardCheck
		refused  bool
		contains string
	}{
		{
			name:    "behind and clean is not refused",
			check:   domain.FastForwardCheck{Branch: "feat", HasUpstream: true, Behind: 2, State: domain.DivergenceBehind},
			refused: false,
		},
		{
			name:    "behind and dirty is not refused: that is a blocker",
			check:   domain.FastForwardCheck{Branch: "feat", HasUpstream: true, Behind: 2, State: domain.DivergenceBehind, IsDirty: true},
			refused: false,
		},
		{
			name:     "no upstream is refused",
			check:    domain.FastForwardCheck{Branch: "feat", State: domain.DivergenceUnknown},
			refused:  true,
			contains: "no origin counterpart",
		},
		{
			name:     "diverged is refused and names sync",
			check:    domain.FastForwardCheck{Branch: "feat", HasUpstream: true, Ahead: 1, Behind: 2, State: domain.DivergenceDiverged},
			refused:  true,
			contains: "wtm sync",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reason, refused := FastForwardRefusal(tc.check)
			if refused != tc.refused {
				t.Fatalf("refused = %v, want %v (reason %q)", refused, tc.refused, reason)
			}
			if tc.contains != "" && !strings.Contains(reason, tc.contains) {
				t.Fatalf("reason = %q, want it to contain %q", reason, tc.contains)
			}
		})
	}
}

func TestFastForwardNeedsWork(t *testing.T) {
	cases := []struct {
		name  string
		check domain.FastForwardCheck
		want  bool
	}{
		{"behind", domain.FastForwardCheck{HasUpstream: true, Behind: 1, State: domain.DivergenceBehind}, true},
		{"up to date", domain.FastForwardCheck{HasUpstream: true, State: domain.DivergenceUpToDate}, false},
		{"ahead only", domain.FastForwardCheck{HasUpstream: true, Ahead: 2, State: domain.DivergenceAhead}, false},
		{"diverged", domain.FastForwardCheck{HasUpstream: true, Ahead: 1, Behind: 1, State: domain.DivergenceDiverged}, false},
		{"no upstream", domain.FastForwardCheck{State: domain.DivergenceUnknown}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FastForwardNeedsWork(tc.check); got != tc.want {
				t.Fatalf("needsWork = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFastForwardReadyBranchesKeepsOnlyTheBehindOnes(t *testing.T) {
	branches := FastForwardReadyBranches([]domain.WorktreeStatus{
		{Branch: "behind", OriginState: domain.DivergenceBehind, OriginBehind: 2},
		{Branch: "current", OriginState: domain.DivergenceUpToDate},
		{Branch: "ahead", OriginState: domain.DivergenceAhead, OriginAhead: 1},
		{Branch: "split", OriginState: domain.DivergenceDiverged, OriginAhead: 1, OriginBehind: 1},
		{Branch: "local", OriginState: domain.DivergenceUnknown},
	})
	if len(branches) != 1 || branches[0] != "behind" {
		t.Fatalf("ready = %v, want [behind]", branches)
	}
}

// A dirty worktree stays checked: the fast-forward is refused later, by name, in
// the recap the user reads — dropping it here would hide a branch that is behind.
func TestFastForwardReadyBranchesKeepsADirtyBehindWorktree(t *testing.T) {
	branches := FastForwardReadyBranches([]domain.WorktreeStatus{
		{Branch: "behind", OriginState: domain.DivergenceBehind, OriginBehind: 1, IsDirty: true},
	})
	if len(branches) != 1 {
		t.Fatalf("ready = %v, want [behind]", branches)
	}
}

func TestFastForwardStatusLabel(t *testing.T) {
	cases := map[domain.FastForwardStatus]string{
		domain.FFUpToDate:   domain.FastForwardLabelUpToDate,
		domain.FFAdvanced:   domain.FastForwardLabelAdvanced,
		domain.FFDiverged:   domain.FastForwardLabelDiverged,
		domain.FFNoUpstream: domain.FastForwardLabelNoRemote,
		domain.FFFailed:     domain.FastForwardLabelFailed,
	}
	for status, want := range cases {
		if got := FastForwardStatusLabel(status); got != want {
			t.Fatalf("label(%v) = %q, want %q", status, got, want)
		}
	}
}
