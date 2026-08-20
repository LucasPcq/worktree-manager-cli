package prune

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/flow"
)

// A purge that fails cannot reach the run — purgeJobLogs returns nothing. The
// flow-level proof is in flow/clean (TestRunSucceedsWhenTheJobLogPurgeFails);
// what is pinned here is that it deletes one worktree's logs and no other's.
func TestPurgeJobLogsDropsOnlyThatWorktreesLogs(t *testing.T) {
	stateDir := t.TempDir()
	f := &pruneFlow{ctx: flow.Context{StateDir: stateDir}}

	pruned := filepath.Join(stateDir, "logs", "feat%2Fdone")
	kept := filepath.Join(stateDir, "logs", "feat%2Fkept")
	for _, dir := range []string{pruned, kept} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create %s: %v", dir, err)
		}
	}

	f.purgeJobLogs("feat/done")

	if _, err := os.Stat(pruned); !os.IsNotExist(err) {
		t.Errorf("the pruned worktree kept its job logs: %v", err)
	}
	if _, err := os.Stat(kept); err != nil {
		t.Errorf("another worktree's job logs were purged: %v", err)
	}
}

func TestPurgeJobLogsToleratesNothingToPurge(t *testing.T) {
	f := &pruneFlow{ctx: flow.Context{StateDir: t.TempDir()}}

	f.purgeJobLogs("feat/never-ran-a-job")
	f.purgeJobLogs("")
}
