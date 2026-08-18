package rules

import (
	"errors"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestParseShell(t *testing.T) {
	for _, name := range []string{"zsh", "bash", "fish", "powershell"} {
		got, err := ParseShell(name)
		if err != nil {
			t.Errorf("ParseShell(%q) returned error: %v", name, err)
		}
		if string(got) != name {
			t.Errorf("ParseShell(%q) = %q", name, got)
		}
	}
}

func TestParseShellUnknown(t *testing.T) {
	if _, err := ParseShell("nushell"); !errors.Is(err, domain.ErrInvalidShellType) {
		t.Errorf("expected ErrInvalidShellType, got %v", err)
	}
}

func TestDefaultShellFor(t *testing.T) {
	if got := DefaultShellFor(domain.GOOSWindows); got != domain.ShellPowerShell {
		t.Errorf("expected powershell on windows, got %s", got)
	}
	if got := DefaultShellFor("darwin"); got != domain.DefaultShell {
		t.Errorf("expected %s on darwin, got %s", domain.DefaultShell, got)
	}
}

func TestShellInitCommandPerShell(t *testing.T) {
	if got := ShellInitCommand(domain.ShellPowerShell); !strings.Contains(got, "Invoke-Expression") {
		t.Errorf("powershell needs Invoke-Expression, got %q", got)
	}
	if got := ShellInitCommand(domain.ShellZsh); !strings.Contains(got, "eval") {
		t.Errorf("zsh needs eval, got %q", got)
	}
}

func TestGenerateShellInitPowerShell(t *testing.T) {
	out := GenerateShellInit(domain.ShellPowerShell)

	for _, want := range []string{
		"function wtm",
		// The function shadows the command name, so the binary must be called
		// by its explicit filename or the wrapper recurses forever.
		"wtm.exe",
		"$args[0] -eq 'go'",
		"$args[0] -eq 'switch'",
		"Set-Location",
		domain.EnvGoFile,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("powershell init missing %q", want)
		}
	}

	for _, unwanted := range []string{"mktemp", "/tmp/"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("powershell init leaks POSIX construct %q", unwanted)
		}
	}
}
