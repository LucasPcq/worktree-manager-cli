package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

const bashZshTemplate = `wtm() {
  if [ "$1" = "go" ]; then
    local dir
    dir="$(command wtm resolve "${@:2}")"
    if [ -n "$dir" ]; then
      cd "$dir" || return 1
    fi
  elif [ "$1" = "switch" ]; then
    local dir flags=() branch_args=()
    shift 1
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
  if test "$argv[1]" = "go"
    set dir (command wtm resolve $argv[2..])
    if test -n "$dir"
      cd "$dir"
    end
  else if test "$argv[1]" = "switch"
    set -l branch_args
    set -l flags
    for arg in $argv[2..]
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

// PowerShell has no `local` scoping quirk to work around, so the two identical
// fallback branches of the POSIX templates (no args / with args) collapse into
// one: splatting an empty @args is a no-op.
const powershellTemplate = `function wtm {
  $exe = 'wtm.exe'
  $rest = @()
  if ($args.Count -gt 1) { $rest = @($args[1..($args.Count - 1)]) }

  if ($args.Count -ge 1 -and $args[0] -eq 'go') {
    $dir = & $exe resolve @rest
    if ($dir) { Set-Location $dir }
    return
  }

  if ($args.Count -ge 1 -and $args[0] -eq 'switch') {
    $flags = @()
    $branchArgs = @()
    $i = 0
    # if/elseif rather than switch: an unlabelled break inside a switch nested in
    # a loop is ambiguous in PowerShell.
    while ($i -lt $rest.Count) {
      $arg = $rest[$i]
      if ($arg -eq '--profile') {
        $flags += $arg
        $i++
        if ($i -lt $rest.Count) { $flags += $rest[$i] }
      }
      elseif ($arg -like '--profile=*' -or $arg -eq '--exclusive' -or $arg -eq '--parallel') {
        $flags += $arg
      }
      else {
        $branchArgs += $arg
      }
      $i++
    }
    $dir = & $exe resolve @branchArgs
    if ($dir) {
      Set-Location $dir
      & $exe run up @flags
    }
    return
  }

  $tmpfile = [System.IO.Path]::GetTempFileName()
  try {
    $env:WTM_GO_FILE = $tmpfile
    & $exe @args
  } finally {
    Remove-Item Env:\WTM_GO_FILE -ErrorAction SilentlyContinue
  }
  if (Test-Path $tmpfile) {
    $dir = Get-Content -Raw -ErrorAction SilentlyContinue $tmpfile
    Remove-Item -Force -ErrorAction SilentlyContinue $tmpfile
    if ($dir) {
      $dir = $dir.Trim()
      if ($dir -and (Test-Path -PathType Container $dir)) { Set-Location $dir }
    }
  }
}
`

// GenerateShellInit returns the shell function wrapper for the given shell type.
func GenerateShellInit(shell domain.ShellType) string {
	switch shell {
	case domain.ShellFish:
		return fishTemplate
	case domain.ShellPowerShell:
		return powershellTemplate
	default:
		return bashZshTemplate
	}
}

// ParseShell resolves a user-supplied shell name to its ShellType.
func ParseShell(name string) (domain.ShellType, error) {
	shell := domain.ShellType(name)
	if err := ValidateShellType(shell); err != nil {
		return "", fmt.Errorf("%w: %s", err, name)
	}
	return shell, nil
}

// DefaultShellFor returns the shell to assume when detection finds nothing,
// given a runtime.GOOS value.
func DefaultShellFor(goos string) domain.ShellType {
	if goos == domain.GOOSWindows {
		return domain.DefaultShellWindows
	}
	return domain.DefaultShell
}

// ShellInitCommand returns the line the user adds to their shell config to load
// the wtm wrapper. PowerShell has no `eval`.
func ShellInitCommand(shell domain.ShellType) string {
	if shell == domain.ShellPowerShell {
		return domain.ShellInitCommandPowerShell
	}
	return domain.ShellInitCommand
}

// ShellInitHint is the multi-line hint printed when a command needs the shell
// wrapper but it is not loaded.
func ShellInitHint(shell domain.ShellType) string {
	return domain.MsgShellInitHintPrefix + ShellInitCommand(shell)
}

// ShellInitNextStep is the single-line variant used in the init recap.
func ShellInitNextStep(shell domain.ShellType) string {
	return domain.InitNextStepShellPrefix + ShellInitCommand(shell)
}
