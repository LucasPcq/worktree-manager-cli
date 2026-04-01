// Package env implements strategies for provisioning .env files in new worktrees.
package env

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
)

// CopyEnvFilesParams holds inputs for provisioning .env files.
type CopyEnvFilesParams struct {
	Strategy           domain.EnvStrategy
	CopyFiles          []string
	TargetDir          string
	MainWorktreePath   string
	ParentWorktreePath string
}

// CopyEnvFiles provisions .env files into a new worktree according to the chosen strategy.
func CopyEnvFiles(params CopyEnvFilesParams) error {
	for _, file := range params.CopyFiles {
		if err := copyEnvFile(params, file); err != nil {
			return err
		}
	}
	return nil
}

func copyEnvFile(params CopyEnvFilesParams, file string) error {
	dst := filepath.Join(params.TargetDir, file)

	switch params.Strategy {
	case domain.EnvStrategyExample:
		return copyFromExample(params.MainWorktreePath, dst, file)
	case domain.EnvStrategyMain:
		return copyFromDir(params.MainWorktreePath, dst, file)
	case domain.EnvStrategyParent:
		return copyFromParent(params, dst, file)
	default:
		return fmt.Errorf("unknown env strategy: %s", params.Strategy)
	}
}

func copyFromExample(mainPath string, dst string, file string) error {
	src := filepath.Join(mainPath, file+".example")
	if !fileExists(src) {
		fmt.Fprintf(os.Stderr, "warning: %s.example not found, skipping\n", file)
		return nil
	}
	return copyFile(src, dst)
}

func copyFromDir(dir string, dst string, file string) error {
	src := filepath.Join(dir, file)
	if !fileExists(src) {
		fmt.Fprintf(os.Stderr, "warning: %s not found in %s, skipping\n", file, dir)
		return nil
	}
	return copyFile(src, dst)
}

func copyFromParent(params CopyEnvFilesParams, dst string, file string) error {
	if params.ParentWorktreePath != "" {
		src := filepath.Join(params.ParentWorktreePath, file)
		if fileExists(src) {
			return copyFile(src, dst)
		}
		fmt.Fprintf(os.Stderr, "warning: %s not found in parent worktree, falling back to main\n", file)
	}
	return copyFromDir(params.MainWorktreePath, dst, file)
}

func copyFile(src string, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", dst, err)
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}

	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", dst, err)
	}

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
