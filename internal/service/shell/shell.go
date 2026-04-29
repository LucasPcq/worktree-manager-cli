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
