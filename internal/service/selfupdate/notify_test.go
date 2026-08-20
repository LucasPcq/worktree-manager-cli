package selfupdate_test

import (
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/selfupdate"
)

// hermeticEnv isolates a notify test from the machine running it: StartCheck
// reads the environment to decide, and CI exports CI/GITHUB_ACTIONS, which would
// suppress every check and make these tests pass locally but fail on CI.
func hermeticEnv(t *testing.T) {
	t.Helper()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv(domain.EnvCI, "")
	t.Setenv(domain.EnvGitHubActions, "")
	t.Setenv(domain.EnvNoUpdateCheck, "")
}

func TestStartCheckSuppressedReturnsNilAndNoticeIsNilSafe(t *testing.T) {
	hermeticEnv(t)
	t.Setenv(domain.EnvNoUpdateCheck, "1")

	check := selfupdate.StartCheck(selfupdate.StartCheckParams{
		Version:     "0.1.0",
		Format:      domain.OutputText,
		Command:     domain.CmdList,
		StderrIsTTY: true,
	})
	if check != nil {
		t.Fatalf("StartCheck = %v, want nil when the opt-out env var is set", check)
	}

	if _, _, _, ok := check.Notice(time.Millisecond); ok {
		t.Fatal("Notice on a nil Check must report ok=false, not panic")
	}
}

func TestNoticeServesTheCachedVersionWithoutNetwork(t *testing.T) {
	hermeticEnv(t)

	// A recent check with a newer version cached: no refresh is due, so the
	// notice must come straight from state with no network call.
	if err := selfupdate.SaveState(domain.UpdateState{
		CheckedAt:     time.Now(),
		LatestVersion: "9.9.9",
	}); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	check := selfupdate.StartCheck(selfupdate.StartCheckParams{
		Version:     "0.1.0",
		Format:      domain.OutputText,
		Command:     domain.CmdList,
		StderrIsTTY: true,
	})
	if check == nil {
		t.Fatal("StartCheck = nil; a TTY run with a newer cached version must arm a notice")
	}

	current, latest, _, ok := check.Notice(time.Millisecond)
	if !ok {
		t.Fatal("Notice reported nothing; a cached newer version must produce a notice")
	}
	if current != "0.1.0" || latest != "9.9.9" {
		t.Fatalf("Notice = (%q, %q), want (0.1.0, 9.9.9)", current, latest)
	}
}

func TestNoticeSilentWhenCachedVersionIsNotNewer(t *testing.T) {
	hermeticEnv(t)

	if err := selfupdate.SaveState(domain.UpdateState{
		CheckedAt:     time.Now(),
		LatestVersion: "0.1.0",
	}); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	check := selfupdate.StartCheck(selfupdate.StartCheckParams{
		Version:     "0.1.0",
		Format:      domain.OutputText,
		Command:     domain.CmdList,
		StderrIsTTY: true,
	})
	if check == nil {
		t.Fatal("StartCheck = nil; the display gate must allow a TTY run")
	}

	if _, _, _, ok := check.Notice(time.Millisecond); ok {
		t.Fatal("Notice fired for an identical version")
	}
}
