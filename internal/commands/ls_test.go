package commands

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
)

func TestBuildWorktreeLabel_SimpleBranch(t *testing.T) {
	s := domain.WorktreeStatus{Branch: "feature/auth"}
	got := buildWorktreeLabel(s, "", nil, nil)
	if got != "feature/auth" {
		t.Errorf("got %q, want %q", got, "feature/auth")
	}
}

func TestBuildWorktreeLabel_Parent(t *testing.T) {
	s := domain.WorktreeStatus{Branch: "main", IsParent: true}
	got := buildWorktreeLabel(s, "", nil, nil)
	want := "main  (parent)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildWorktreeLabel_Active(t *testing.T) {
	s := domain.WorktreeStatus{Branch: "develop"}
	got := buildWorktreeLabel(s, "develop", nil, nil)
	want := "develop  (active)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildWorktreeLabel_WithPR(t *testing.T) {
	s := domain.WorktreeStatus{Branch: "feature/auth"}
	prs := []domain.PRInfo{{Number: 42, Branch: "feature/auth"}}
	got := buildWorktreeLabel(s, "", prs, nil)
	want := "feature/auth  (PR #42)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildWorktreeLabel_Dirty(t *testing.T) {
	s := domain.WorktreeStatus{Branch: "feature/auth", IsDirty: true}
	got := buildWorktreeLabel(s, "", nil, nil)
	want := "feature/auth  (dirty)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildWorktreeLabel_MultipleTags(t *testing.T) {
	s := domain.WorktreeStatus{
		Branch:   "main",
		Path:     "/tmp/wt/main",
		IsParent: true,
		IsDirty:  true,
	}
	prs := []domain.PRInfo{{Number: 42, Branch: "main"}}
	services := []process.ServiceInfo{{
		Name:    "api",
		Status:  domain.ServiceStatusRunning,
		WorkDir: "/tmp/wt/main",
	}}
	got := buildWorktreeLabel(s, "main", prs, services)
	want := "main  (parent, active, PR #42, services, dirty)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestJoinTags_Empty(t *testing.T) {
	if got := joinTags(nil); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestJoinTags_One(t *testing.T) {
	if got := joinTags([]string{"parent"}); got != "parent" {
		t.Errorf("got %q, want %q", got, "parent")
	}
}

func TestJoinTags_Multiple(t *testing.T) {
	got := joinTags([]string{"parent", "active", "dirty"})
	want := "parent, active, dirty"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
