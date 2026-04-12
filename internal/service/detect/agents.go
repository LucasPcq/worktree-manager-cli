package detect

import (
	"os"
	"path/filepath"
)

// AgentKind identifies the destination type for `wtm agents install`.
type AgentKind string

const (
	AgentKindClaudeProject AgentKind = "claude-project"
	AgentKindClaudeGlobal  AgentKind = "claude-global"
	AgentKindCursorProject AgentKind = "cursor-project"
	AgentKindCursorGlobal  AgentKind = "cursor-global"
)

// AgentTarget describes a skill destination `wtm agents install` can write to.
// Path points to the SKILL.md that will be written.
type AgentTarget struct {
	Kind     AgentKind
	Path     string
	Exists   bool
	IsGlobal bool
}

// AgentTargets scans the project and the user's home directory for skill
// destinations (.claude / .cursor, project and global). `Exists` reflects
// whether the parent directory is already there — the SKILL.md itself may
// or may not exist yet.
func AgentTargets(projectDir string) []AgentTarget {
	home, _ := os.UserHomeDir()

	targets := []struct {
		kind     AgentKind
		marker   string
		artifact string
		global   bool
	}{
		{AgentKindClaudeProject, filepath.Join(projectDir, ".claude"), filepath.Join(projectDir, ".claude", "skills", "using-wtm", "SKILL.md"), false},
		{AgentKindCursorProject, filepath.Join(projectDir, ".cursor"), filepath.Join(projectDir, ".cursor", "skills", "using-wtm", "SKILL.md"), false},
	}

	if home != "" {
		targets = append(targets,
			struct {
				kind     AgentKind
				marker   string
				artifact string
				global   bool
			}{AgentKindClaudeGlobal, filepath.Join(home, ".claude"), filepath.Join(home, ".claude", "skills", "using-wtm", "SKILL.md"), true},
			struct {
				kind     AgentKind
				marker   string
				artifact string
				global   bool
			}{AgentKindCursorGlobal, filepath.Join(home, ".cursor"), filepath.Join(home, ".cursor", "skills", "using-wtm", "SKILL.md"), true},
		)
	}

	out := make([]AgentTarget, 0, len(targets))
	for _, t := range targets {
		_, err := os.Stat(t.marker)
		out = append(out, AgentTarget{
			Kind:     t.kind,
			Path:     t.artifact,
			Exists:   err == nil,
			IsGlobal: t.global,
		})
	}
	return out
}
