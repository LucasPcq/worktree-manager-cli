## wtm shell-init

Generate shell integration function

### Synopsis

Output a shell function to eval in your rc file.

The shell is detected from $SHELL; pass it explicitly when detection
cannot work — notably PowerShell, which does not set $SHELL.

Usage: eval "$(wtm shell-init)"
PowerShell: Invoke-Expression (& wtm shell-init | Out-String)

```
wtm shell-init [zsh|bash|fish|powershell] [flags]
```

### Options

```
  -h, --help   help for shell-init
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

