package domain

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
