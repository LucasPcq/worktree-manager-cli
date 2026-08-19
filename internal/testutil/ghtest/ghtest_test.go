package ghtest_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/testutil/ghtest"
)

// shareADirectory puts a fake `gh` and a `git` in one directory at the head of
// PATH, which is how a CI runner is laid out (/usr/bin holds both). A developer
// machine usually keeps them apart, so without this the case below never occurs
// locally — and the first version of Absent, which dropped the whole entry, took
// git down with it on CI only.
func shareADirectory(t *testing.T) {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Skipf("no git to mirror: %v", err)
	}

	shared := t.TempDir()
	if err := os.WriteFile(filepath.Join(shared, "gh"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write gh: %v", err)
	}
	if err := os.Symlink(realGit, filepath.Join(shared, "git")); err != nil {
		t.Fatalf("link git: %v", err)
	}
	t.Setenv("PATH", shared)
}

func TestAbsentHidesGH(t *testing.T) {
	shareADirectory(t)
	ghtest.Absent(t)

	if path, err := exec.LookPath("gh"); err == nil {
		t.Errorf("gh still resolves to %s, want it hidden", path)
	}
}

func TestAbsentKeepsEverythingElseOnPath(t *testing.T) {
	shareADirectory(t)
	ghtest.Absent(t)

	if _, err := exec.LookPath("git"); err != nil {
		t.Errorf("git must still resolve after hiding gh: %v", err)
	}
}

func TestStubAnswersAuthAndPRList(t *testing.T) {
	ghtest.Stub(t, ghtest.StubParams{PRs: []ghtest.PR{{Number: 7, Branch: "feat", State: "merged"}}})

	if err := exec.Command("gh", "auth", "status").Run(); err != nil {
		t.Errorf("the stub must authenticate by default: %v", err)
	}
	out, err := exec.Command("gh", "pr", "list", "--state", "all").Output()
	if err != nil {
		t.Fatalf("pr list: %v", err)
	}
	if !strings.HasPrefix(string(out), "[") {
		t.Errorf("pr list = %q, want a JSON array", out)
	}
}

func TestStubCanRefuseAuthentication(t *testing.T) {
	ghtest.Stub(t, ghtest.StubParams{Unauthenticated: true})

	if err := exec.Command("gh", "auth", "status").Run(); err == nil {
		t.Error("an unauthenticated stub must fail `gh auth status`")
	}
}
