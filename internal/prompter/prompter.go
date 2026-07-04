// Package prompter holds the GENERIC user-INPUT primitives — soliciting a decision
// from the user (Confirm, and future Input/Select). Output feedback (spinners) is a
// separate concern: see internal/progress. Command-specific composite flows (e.g.
// clean's wizard) do NOT belong here either; they use a port next to their command
// (see internal/cmd/clean/wizard). The only concrete implementation lives in
// prompter/bubbletea; tests inject a mock.
package prompter

// Prompter is the generic user-input contract shared across commands.
type Prompter interface {
	// Confirm asks a single yes/no question. For destructive actions callers pass
	// defaultYes=false so Enter cancels.
	Confirm(prompt string, defaultYes bool) (bool, error)
}
