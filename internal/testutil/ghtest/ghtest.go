// Package ghtest drives the GitHub CLI a test sees. `service/github` shells out
// to `gh`, so a script first on PATH is enough to script PR state without a
// network, and dropping every PATH entry that holds one is enough to simulate
// its absence.
package ghtest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// PR is one pull request the stubbed `gh pr list --state all` reports.
type PR struct {
	Number int
	Branch string
	State  string
}

type StubParams struct {
	PRs []PR
	// Unauthenticated makes `gh auth status` fail, which the service maps to
	// domain.GHConnectionNotAuthenticated.
	Unauthenticated bool
}

// ghPR is the shape service/github parses out of `gh pr list --json`.
type ghPR struct {
	Number      int    `json:"number"`
	HeadRefName string `json:"headRefName"`
	URL         string `json:"url"`
	State       string `json:"state"`
}

// Stub puts a fake `gh` first on PATH for the rest of the test.
func Stub(t testing.TB, params StubParams) {
	t.Helper()

	items := make([]ghPR, 0, len(params.PRs))
	for _, pr := range params.PRs {
		items = append(items, ghPR{
			Number:      pr.Number,
			HeadRefName: pr.Branch,
			URL:         fmt.Sprintf("https://github.com/test/test/pull/%d", pr.Number),
			State:       strings.ToUpper(pr.State),
		})
	}
	payload, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("marshal stub PRs: %v", err)
	}

	authExit := 0
	if params.Unauthenticated {
		authExit = 1
	}

	dir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
case "$1" in
  auth) exit %d ;;
  pr)
    cat <<'WTM_GH_STUB_EOF'
%s
WTM_GH_STUB_EOF
    exit 0 ;;
esac
exit 1
`, authExit, payload)

	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatalf("write gh stub: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// Absent hides `gh` from exec.LookPath by dropping the PATH entries that hold
// one, rather than by replacing PATH — everything else the test shells out to
// (git first of all) keeps resolving exactly as it did.
func Absent(t testing.TB) {
	t.Helper()

	kept := make([]string, 0)
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		if info, err := os.Stat(filepath.Join(dir, "gh")); err == nil && !info.IsDir() {
			continue
		}
		kept = append(kept, dir)
	}
	t.Setenv("PATH", strings.Join(kept, string(os.PathListSeparator)))
}
