package shell

import (
	"os"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestGenerateShellInitZsh(t *testing.T) {
	out := GenerateShellInit(domain.ShellZsh)

	if !strings.Contains(out, "wtm()") {
		t.Error("expected bash/zsh function definition")
	}
	if !strings.Contains(out, `command wtm resolve`) {
		t.Error("expected resolve call")
	}
	if !strings.Contains(out, `cd "$dir"`) {
		t.Error("expected cd command")
	}
}

func TestGenerateShellInitBash(t *testing.T) {
	out := GenerateShellInit(domain.ShellBash)

	if !strings.Contains(out, "wtm()") {
		t.Error("expected bash function definition")
	}
}

func TestGenerateShellInitFish(t *testing.T) {
	out := GenerateShellInit(domain.ShellFish)

	if !strings.Contains(out, "function wtm") {
		t.Error("expected fish function definition")
	}
	if !strings.Contains(out, "command wtm resolve") {
		t.Error("expected resolve call")
	}
}

func TestDetectShellZsh(t *testing.T) {
	os.Setenv("SHELL", "/bin/zsh")
	defer os.Unsetenv("SHELL")

	if got := DetectShell(); got != domain.ShellZsh {
		t.Errorf("expected zsh, got %s", got)
	}
}

func TestDetectShellBash(t *testing.T) {
	os.Setenv("SHELL", "/usr/bin/bash")
	defer os.Unsetenv("SHELL")

	if got := DetectShell(); got != domain.ShellBash {
		t.Errorf("expected bash, got %s", got)
	}
}

func TestDetectShellFish(t *testing.T) {
	os.Setenv("SHELL", "/opt/homebrew/bin/fish")
	defer os.Unsetenv("SHELL")

	if got := DetectShell(); got != domain.ShellFish {
		t.Errorf("expected fish, got %s", got)
	}
}

func TestDetectShellDefault(t *testing.T) {
	os.Setenv("SHELL", "")
	defer os.Unsetenv("SHELL")

	if got := DetectShell(); got != domain.DefaultShell {
		t.Errorf("expected default %s, got %s", domain.DefaultShell, got)
	}
}
