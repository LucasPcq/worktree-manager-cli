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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DecidePush(tt.params); got != tt.want {
				t.Fatalf("DecidePush = %v, want %v", got, tt.want)
			}
		})
	}
}
