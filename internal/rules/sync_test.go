package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestSyncIncludesBase(t *testing.T) {
	tests := []struct {
		name     string
		selected []string
		base     string
		want     bool
	}{
		{"nil selection means all", nil, "main", true},
		{"base explicitly selected", []string{"feat", "main"}, "main", true},
		{"base not selected", []string{"feat"}, "main", false},
		{"empty non-nil selection", []string{}, "main", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SyncIncludesBase(SyncIncludesBaseParams{Selected: tt.selected, BaseBranch: tt.base})
			if got != tt.want {
				t.Fatalf("SyncIncludesBase(%v, %q) = %v, want %v", tt.selected, tt.base, got, tt.want)
			}
		})
	}
}

func TestPushableCount(t *testing.T) {
	steps := []domain.SyncStepResult{
		{PushPending: true, Pushed: false},  // counts
		{PushPending: true, Pushed: true},   // already pushed
		{PushPending: false, Pushed: false}, // nothing to push
		{PushPending: true, Pushed: false},  // counts
	}
	if got := PushableCount(steps); got != 2 {
		t.Fatalf("PushableCount = %d, want 2", got)
	}
	if got := PushableCount(nil); got != 0 {
		t.Fatalf("PushableCount(nil) = %d, want 0", got)
	}
}

func TestHasSyncFailure(t *testing.T) {
	tests := []struct {
		name  string
		steps []domain.SyncStepResult
		want  bool
	}{
		{"empty", nil, false},
		{"all ok", []domain.SyncStepResult{{Status: domain.SyncStatusSynced}}, false},
		{"conflict", []domain.SyncStepResult{{Status: domain.SyncStatusConflict}}, true},
		{"error", []domain.SyncStepResult{{Status: domain.SyncStatusError}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasSyncFailure(tt.steps); got != tt.want {
				t.Fatalf("HasSyncFailure = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDecidePush(t *testing.T) {
	tests := []struct {
		name   string
		params DecidePushParams
		want   PushDecision
	}{
		{"nothing pushable", DecidePushParams{Push: true, PushableCount: 0}, PushSkip},
		{"no-push wins", DecidePushParams{Push: true, NoPush: true, PushableCount: 2}, PushSkip},
		{"force with push flag", DecidePushParams{Push: true, PushableCount: 2}, PushForce},
		{"non-interactive without push", DecidePushParams{Interactive: false, PushableCount: 2}, PushSkip},
		{"interactive asks", DecidePushParams{Interactive: true, PushableCount: 2}, PushConfirm},
		{"yes without push skips", DecidePushParams{Yes: true, Interactive: true, PushableCount: 2}, PushSkip},
		{"yes with push forces", DecidePushParams{Yes: true, Push: true, Interactive: true, PushableCount: 2}, PushForce},
		{"yes with no-push skips", DecidePushParams{Yes: true, NoPush: true, Interactive: true, PushableCount: 2}, PushSkip},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DecidePush(tt.params); got != tt.want {
				t.Fatalf("DecidePush = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDecideParentFastForward(t *testing.T) {
	tests := []struct {
		name string
		in   DecideParentFastForwardParams
		want ParentDecision
	}{
		{name: "nothing stale", in: DecideParentFastForwardParams{Interactive: true}, want: ParentLeaveAsIs},
		{name: "no-ff wins", in: DecideParentFastForwardParams{NoFF: true, FF: true, StaleCount: 1}, want: ParentLeaveAsIs},
		{name: "ff forces", in: DecideParentFastForwardParams{FF: true, StaleCount: 1}, want: ParentFastForward},
		{name: "ff wins over yes", in: DecideParentFastForwardParams{FF: true, Yes: true, StaleCount: 1}, want: ParentFastForward},
		{name: "yes leaves as is", in: DecideParentFastForwardParams{Yes: true, Interactive: true, StaleCount: 2}, want: ParentLeaveAsIs},
		{name: "non-interactive leaves as is", in: DecideParentFastForwardParams{StaleCount: 2}, want: ParentLeaveAsIs},
		{name: "interactive asks", in: DecideParentFastForwardParams{Interactive: true, StaleCount: 2}, want: ParentAsk},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := DecideParentFastForward(tc.in); got != tc.want {
				t.Fatalf("DecideParentFastForward(%+v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestStaleParents(t *testing.T) {
	updates := []domain.ParentUpdate{
		{Branch: "a", Status: domain.ParentBehind},
		{Branch: "b", Status: domain.ParentDiverged},
		{Branch: "c", Status: domain.ParentFastForwarded},
		{Branch: "d", Status: domain.ParentBehind},
	}
	stale := StaleParents(updates)
	if len(stale) != 2 || stale[0].Branch != "a" || stale[1].Branch != "d" {
		t.Fatalf("StaleParents = %+v, want the two behind parents", stale)
	}
}

func TestCommitCountLabel(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{n: 0, want: "0 commits"},
		{n: 1, want: "1 commit"},
		{n: 2, want: "2 commits"},
	}
	for _, tc := range tests {
		if got := CommitCountLabel(tc.n); got != tc.want {
			t.Errorf("CommitCountLabel(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}
