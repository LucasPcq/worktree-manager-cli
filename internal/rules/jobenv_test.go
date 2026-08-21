package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestWorktreeSlug(t *testing.T) {
	cases := []struct {
		name   string
		branch string
		want   string
	}{
		{"simple", "main", "main"},
		{"slashes", "feat/isolation", "feat-isolation"},
		{"majuscules refusées par compose", "feat/LUC-99", "feat-luc-99"},
		{"underscore conservé", "feat/my_branch", "feat-my_branch"},
		{"caractères exotiques", "feat/été+2026", "feat--t--2026"},
		{"préfixe non alphanumérique", "-/-feat", "feat"},
		{"vide", "", domain.ComposeProjectFallback},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := WorktreeSlug(c.branch); got != c.want {
				t.Errorf("WorktreeSlug(%q) = %q, want %q", c.branch, got, c.want)
			}
		})
	}
}

func TestWorktreeJobEnv(t *testing.T) {
	cases := []struct {
		name   string
		params WorktreeJobEnvParams
		want   map[string]string
	}{
		{
			name:   "worktree principal garde les ports par défaut",
			params: WorktreeJobEnvParams{Branch: "main", Ordinal: 0, PortOffsetBlock: 10},
			want: map[string]string{
				domain.EnvWorktree:           "main",
				domain.EnvBranch:             "main",
				domain.EnvOrdinal:            "0",
				domain.EnvPortOffset:         "0",
				domain.EnvComposeProjectName: "main",
			},
		},
		{
			name:   "offset = ordinal x bloc",
			params: WorktreeJobEnvParams{Branch: "feat/x", Ordinal: 3, PortOffsetBlock: 10},
			want: map[string]string{
				domain.EnvWorktree:           "feat-x",
				domain.EnvBranch:             "feat/x",
				domain.EnvOrdinal:            "3",
				domain.EnvPortOffset:         "30",
				domain.EnvComposeProjectName: "feat-x",
			},
		},
		{
			name:   "bloc absent retombe sur le défaut",
			params: WorktreeJobEnvParams{Branch: "feat/x", Ordinal: 2},
			want: map[string]string{
				domain.EnvWorktree:           "feat-x",
				domain.EnvBranch:             "feat/x",
				domain.EnvOrdinal:            "2",
				domain.EnvPortOffset:         "20",
				domain.EnvComposeProjectName: "feat-x",
			},
		},
		{
			name:   "COMPOSE_PROJECT_NAME défini par l'utilisateur non écrasé",
			params: WorktreeJobEnvParams{Branch: "feat/x", Ordinal: 1, PortOffsetBlock: 10, ComposeProject: "perso"},
			want: map[string]string{
				domain.EnvWorktree:           "feat-x",
				domain.EnvBranch:             "feat/x",
				domain.EnvOrdinal:            "1",
				domain.EnvPortOffset:         "10",
				domain.EnvComposeProjectName: "perso",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := WorktreeJobEnv(c.params)
			if len(got) != len(c.want) {
				t.Fatalf("WorktreeJobEnv() = %v, want %v", got, c.want)
			}
			for key, want := range c.want {
				if got[key] != want {
					t.Errorf("WorktreeJobEnv()[%s] = %q, want %q", key, got[key], want)
				}
			}
		})
	}
}

func TestPurgeableMetaDir(t *testing.T) {
	cases := []struct {
		name     string
		stateDir string
		branch   string
		want     string
	}{
		{"branche normale", "/state", "feat/x", "/state/worktrees/feat%2Fx"},
		{"state dir absent", "", "feat/x", ""},
		{"branche absente", "/state", "", ""},
		{"branche remontant d'un cran", "/state", "..", ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := PurgeableMetaDir(PurgeableMetaDirParams{StateDir: c.stateDir, Branch: c.branch})
			if got != c.want {
				t.Errorf("PurgeableMetaDir(%q, %q) = %q, want %q", c.stateDir, c.branch, got, c.want)
			}
		})
	}
}
