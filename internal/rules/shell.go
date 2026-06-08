package rules

import "github.com/LucasPcq/wtm/internal/domain"

const bashZshTemplate = `wtm() {
  if [ "$1" = "wt" ] && [ "$2" = "go" ]; then
    local dir
    dir="$(command wtm resolve "${@:3}")"
    if [ -n "$dir" ]; then
      cd "$dir" || return 1
    fi
  elif [ "$1" = "wt" ] && [ "$2" = "switch" ]; then
    local dir flags=() branch_args=()
    shift 2
    while [ $# -gt 0 ]; do
      case "$1" in
        --exclusive|--parallel|--profile|--profile=*) flags+=("$1"); [ "$1" = "--profile" ] && { shift; flags+=("$1"); } ;;
        *) branch_args+=("$1") ;;
      esac
      shift
    done
    dir="$(command wtm resolve "${branch_args[@]}")"
    if [ -n "$dir" ]; then
      cd "$dir" || return 1
      command wtm run up "${flags[@]}"
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
    local tmpfile
    tmpfile="$(mktemp /tmp/wtm-go.XXXXXX)"
    WTM_GO_FILE="$tmpfile" command wtm "$@"
    if [ -f "$tmpfile" ]; then
      local dir
      dir="$(cat "$tmpfile")"
      rm -f "$tmpfile"
      if [ -n "$dir" ] && [ -d "$dir" ]; then
        cd "$dir" || return 1
      fi
    fi
  fi
}
`

const fishTemplate = `function wtm
  if test "$argv[1]" = "wt" -a "$argv[2]" = "go"
    set dir (command wtm resolve $argv[3..])
    if test -n "$dir"
      cd "$dir"
    end
  else if test "$argv[1]" = "wt" -a "$argv[2]" = "switch"
    set -l branch_args
    set -l flags
    for arg in $argv[3..]
      switch $arg
        case '--exclusive' '--parallel' '--profile' '--profile=*'
          set flags $flags $arg
        case '*'
          set branch_args $branch_args $arg
      end
    end
    set dir (command wtm resolve $branch_args)
    if test -n "$dir"
      cd "$dir"
      command wtm run up $flags
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
    set tmpfile (mktemp /tmp/wtm-go.XXXXXX)
    WTM_GO_FILE="$tmpfile" command wtm $argv
    if test -f "$tmpfile"
      set dir (cat "$tmpfile")
      rm -f "$tmpfile"
      if test -n "$dir" -a -d "$dir"
        cd "$dir"
      end
    end
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
