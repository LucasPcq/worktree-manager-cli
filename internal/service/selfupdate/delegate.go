package selfupdate

import (
	"io"
	"os/exec"

	"github.com/LucasPcq/wtm/internal/domain"
)

type DelegateParams struct {
	Method domain.InstallMethod
	Stdout io.Writer
	Stderr io.Writer
}

// Delegate hands the upgrade to the package manager that owns the binary.
// It reports ran=false when that tool is absent from PATH, so the caller can
// print the command instead of failing.
func Delegate(params DelegateParams) (ran bool, err error) {
	commands, ok := delegatedCommands(params.Method)
	if !ok {
		return false, nil
	}

	if _, err := exec.LookPath(commands[0][0]); err != nil {
		return false, nil
	}

	for _, argv := range commands {
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Stdout = params.Stdout
		cmd.Stderr = params.Stderr
		if err := cmd.Run(); err != nil {
			return true, err
		}
	}

	return true, nil
}

func delegatedCommands(method domain.InstallMethod) ([][]string, bool) {
	switch method {
	case domain.InstallHomebrew:
		// brew update first: without it the tap never sees the new formula.
		return [][]string{
			{"brew", "update"},
			{"brew", "upgrade", domain.BrewFormula},
		}, true
	case domain.InstallGoInstall:
		return [][]string{
			{"go", "install", domain.ModulePath + "@latest"},
		}, true
	default:
		return nil, false
	}
}
