package wt

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// Without a TTY, in human output and with neither --yes nor --dry-run, sync
// refuses instead of attempting a confirmation that cannot be displayed. This is
// prune's model (PruneNeedsTerminal), and the only behavior the flow migration
// changes.
func TestSyncWithoutTerminalRefuses(t *testing.T) {
	setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})

	_, _, err := runWtCmd(t, domain.CmdSync, "feat-a")
	if err == nil {
		t.Fatal("sync without a terminal and without --yes must be refused")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagYes) {
		t.Fatalf("the refusal must name --yes, got: %v", err)
	}
}
