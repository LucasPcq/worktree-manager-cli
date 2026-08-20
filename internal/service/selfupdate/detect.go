package selfupdate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type Install struct {
	Method domain.InstallMethod
	// BinaryPath is the resolved path, so a replacement targets the real file
	// rather than a symlink pointing at it.
	BinaryPath string
}

func DetectInstall(version string) Install {
	execPath, err := os.Executable()
	if err != nil {
		return Install{Method: domain.InstallStandalone}
	}

	resolved, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		resolved = execPath
	}

	return Install{
		Method: rules.ClassifyInstall(rules.ClassifyInstallParams{
			ResolvedPath: resolved,
			// Lazy: resolving it shells out to `go env`, which a Homebrew or
			// source install never needs to pay for.
			GoBinDir: goBinDir,
			Version:  version,
		}),
		BinaryPath: resolved,
	}
}

func goBinDir() string {
	if bin := os.Getenv("GOBIN"); bin != "" {
		return bin
	}

	out, err := exec.Command("go", "env", "GOBIN", "GOPATH").Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) == 2 {
			if lines[0] != "" {
				return lines[0]
			}
			if lines[1] != "" {
				return filepath.Join(lines[1], "bin")
			}
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, "go", "bin")
}
