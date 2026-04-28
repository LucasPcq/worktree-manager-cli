package detect

import (
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
)

// AgentTargets scans the project and the user's home directory for skill
// destinations (.claude / .cursor, project and global). Exists reflects
// whether the parent directory is already there — the SKILL.md itself may
// or may not exist yet.
func AgentTargets(projectDir string) []domain.AgentTarget {
	home, _ := os.UserHomeDir()

	targets := []struct {
		kind     domain.AgentKind
		marker   string
		artifact string
		global   bool
	}{
		{domain.AgentKindClaudeProject, filepath.Join(projectDir, ".claude"), filepath.Join(projectDir, ".claude", "skills", "using-wtm", "SKILL.md"), false},
		{domain.AgentKindCursorProject, filepath.Join(projectDir, ".cursor"), filepath.Join(projectDir, ".cursor", "skills", "using-wtm", "SKILL.md"), false},
	}

	if home != "" {
		targets = append(targets,
			struct {
				kind     domain.AgentKind
				marker   string
				artifact string
				global   bool
			}{domain.AgentKindClaudeGlobal, filepath.Join(home, ".claude"), filepath.Join(home, ".claude", "skills", "using-wtm", "SKILL.md"), true},
			struct {
				kind     domain.AgentKind
				marker   string
				artifact string
				global   bool
			}{domain.AgentKindCursorGlobal, filepath.Join(home, ".cursor"), filepath.Join(home, ".cursor", "skills", "using-wtm", "SKILL.md"), true},
		)
	}

	out := make([]domain.AgentTarget, 0, len(targets))
	for _, t := range targets {
		_, err := os.Stat(t.marker)
		out = append(out, domain.AgentTarget{
			Kind:     t.kind,
			Path:     t.artifact,
			Exists:   err == nil,
			IsGlobal: t.global,
		})
	}
	return out
}
