// Package shell generates shell integration functions for zsh, bash, and fish.
package shell

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// DetectShell reads $SHELL and returns the matching ShellType.
// Defaults to ShellZsh if detection fails.
func DetectShell() domain.ShellType {
	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		return domain.DefaultShell
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

// RequestCd asks the shell wrapper to cd into dir once the command exits, through
// the WTM_GO_FILE bridge (see `wtm shell-init`). A no-op without the bridge, so a
// caller can always ask. Errors are ignored: the directory change is a
// convenience, never the point of the command.
func RequestCd(dir string) {
	goFile := os.Getenv(domain.EnvGoFile)
	if goFile == "" {
		return
	}
	_ = os.WriteFile(goFile, []byte(dir), 0o644)
}
