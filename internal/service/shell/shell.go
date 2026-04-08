// Package shell generates shell integration functions for zsh, bash, and fish.
package shell

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

const bashZshTemplate = `wtm() {
  if [ "$1" = "wt" ] && [ "$2" = "go" ]; then
    local dir
    dir="$(command wtm resolve "${@:3}")"
    if [ -n "$dir" ]; then
      cd "$dir" || return 1
    fi
  elif [ "$1" = "wt" ] && [ "$2" = "clean" ]; then
    local root
    root="$(git worktree list --porcelain 2>/dev/null | head -1 | sed 's/^worktree //')"
    command wtm "$@"
    if [ ! -d "$PWD" ]; then
      if [ -n "$root" ] && [ -d "$root" ]; then
        echo "Worktree removed. Returning to $root"
        cd "$root"
      fi
    fi
  elif [ $# -eq 0 ]; then
    local tmpfile
    tmpfile="$(mktemp /tmp/wtm-go.XXXXXX)"
    WTM_GO_FILE="$tmpfile" command wtm
    if [ -f "$tmpfile" ]; then
      local dir
      dir="$(cat "$tmpfile")"
      rm -f "$tmpfile"
      if [ -n "$dir" ] && [ -d "$dir" ]; then
        cd "$dir" || return 1
      fi
    fi
  else
    command wtm "$@"
  fi
}
`

const fishTemplate = `function wtm
  if test "$argv[1]" = "wt" -a "$argv[2]" = "go"
    set dir (command wtm resolve $argv[3..])
    if test -n "$dir"
      cd "$dir"
    end
  else if test "$argv[1]" = "wt" -a "$argv[2]" = "clean"
    set root (git worktree list --porcelain 2>/dev/null | head -1 | sed 's/^worktree //')
    command wtm $argv
    if not test -d "$PWD"
      if test -n "$root" -a -d "$root"
        echo "Worktree removed. Returning to $root"
        cd "$root"
      end
    end
  else if test (count $argv) -eq 0
    set tmpfile (mktemp /tmp/wtm-go.XXXXXX)
    WTM_GO_FILE="$tmpfile" command wtm
    if test -f "$tmpfile"
      set dir (cat "$tmpfile")
      rm -f "$tmpfile"
      if test -n "$dir" -a -d "$dir"
        cd "$dir"
      end
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
