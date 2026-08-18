// Package shell generates shell integration functions for zsh, bash, fish and PowerShell.
package shell

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// DetectShell reads $SHELL and returns the matching ShellType. When $SHELL is
// absent it falls back to the platform default — PowerShell on Windows, zsh
// elsewhere. Git Bash and WSL both export $SHELL, so they still detect as bash.
func DetectShell() domain.ShellType {
	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		return rules.DefaultShellFor(runtime.GOOS)
	}

	name := filepath.Base(shellPath)

	switch {
	case strings.Contains(name, "fish"):
		return domain.ShellFish
	case strings.Contains(name, "bash"):
		return domain.ShellBash
	default:
		return domain.ShellZsh
	}
}
