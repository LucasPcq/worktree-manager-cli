// Package shell generates shell integration functions for zsh, bash, and fish.
package shell

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

const bashZshTemplate = `wtm() {
  if [ "$1" = "go" ]; then
    local dir
    dir="$(command wtm resolve "${@:2}")"
    if [ -n "$dir" ]; then
      cd "$dir" || return 1
    fi
  else
    command wtm "$@"
  fi
}
`

const fishTemplate = `function wtm
  if test "$argv[1]" = "go"
    set dir (command wtm resolve $argv[2..])
    if test -n "$dir"
      cd "$dir"
    end
  else
    command wtm $argv
  end
end
`

// GenerateShellInit returns the shell function wrapper for the given shell type.
func GenerateShellInit(shell domain.ShellType) string {
	switch shell {
	case domain.ShellFish:
		return fishTemplate
	default:
		return bashZshTemplate
	}
}

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
