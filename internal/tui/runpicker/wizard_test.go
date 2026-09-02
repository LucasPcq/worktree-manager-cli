package runpicker

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// A profile is chosen by what it starts as much as by its name, so its jobs are
// part of the label.
func TestProfileItemsNameWhatEachProfileStarts(t *testing.T) {
	items := profileItems([]domain.ProfileConfig{
		{Name: "app", Jobs: []string{"api", "web", "worker"}},
		{Name: "bare"},
	})

	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if want := "app (api, web, worker)"; items[0].Label != want {
		t.Errorf("label %q, want %q", items[0].Label, want)
	}
	if items[1].Label != "bare" {
		t.Errorf("a profile with no job listed should read as its name alone, got %q", items[1].Label)
	}
	if items[0].Value != "app" {
		t.Errorf("the value must stay the profile name, got %q", items[0].Value)
	}
}

// The badge is the whole reason the worktree step is a view rather than a toll:
// it says where something is already running.
func TestWorktreeItemsAnnounceWhatIsRunning(t *testing.T) {
	items := worktreeItems(WorktreeStep{
		Worktrees: []domain.GitWorktree{{Path: "/a", Branch: "main"}, {Path: "/b", Branch: "feat/x"}},
		Current:   "/a",
		Running:   map[string]int{"/b": 2},
	})

	if got := badgeText(items[0].Badges); !strings.Contains(got, domain.RunWorktreeCurrent) {
		t.Errorf("the current worktree is not marked: %q", got)
	}
	if got := badgeText(items[1].Badges); !strings.Contains(got, "2") {
		t.Errorf("a worktree with jobs up does not say so: %q", got)
	}
	if got := badgeText(items[0].Badges); strings.Contains(got, "running") {
		t.Errorf("a worktree with nothing up claims otherwise: %q", got)
	}
}

func badgeText(badges []components.Badge) string {
	texts := make([]string, 0, len(badges))
	for _, b := range badges {
		texts = append(texts, b.Text)
	}
	return strings.Join(texts, " ")
}
