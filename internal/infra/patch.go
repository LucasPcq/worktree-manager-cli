package infra

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DiffFilesParams holds inputs for producing a patch of tracked changes.
type DiffFilesParams struct {
	WorktreePath string
	Files        []string
}

// DiffFiles returns a binary-safe patch of the working-tree changes (staged and
// unstaged) for the given tracked files, relative to HEAD. The patch reproduces
// the change when applied onto another worktree. Returns an empty slice when
// Files is empty.
func DiffFiles(params DiffFilesParams) ([]byte, error) {
	if len(params.Files) == 0 {
		return nil, nil
	}

	args := append([]string{"-C", params.WorktreePath, "diff", "HEAD", "--binary", "--"}, params.Files...)
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}
	return out, nil
}

// ApplyPatchParams holds inputs for applying a patch to a worktree.
type ApplyPatchParams struct {
	WorktreePath string
	Patch        []byte
	ThreeWay     bool
	Reverse      bool
	Check        bool
}

// ApplyPatch runs `git apply` against the worktree, reading the patch from
// stdin. Check performs a dry run (no changes). ThreeWay enables the 3-way merge
// fallback. Reverse undoes the patch. A non-nil error means the patch did not
// apply cleanly.
func ApplyPatch(params ApplyPatchParams) error {
	if len(params.Patch) == 0 {
		return nil
	}

	args := []string{"-C", params.WorktreePath, "apply", "--whitespace=nowarn"}
	if params.ThreeWay {
		args = append(args, "--3way")
	}
	if params.Reverse {
		args = append(args, "--reverse")
	}
	if params.Check {
		args = append(args, "--check")
	}

	cmd := exec.Command("git", args...)
	cmd.Stdin = bytes.NewReader(params.Patch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git apply: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// ResetPathsParams holds inputs for unstaging paths.
type ResetPathsParams struct {
	WorktreePath string
	Files        []string
}

// ResetPaths unstages the given paths (`git reset -- <paths>`) so the index
// matches HEAD after the working tree has been reverted.
func ResetPaths(params ResetPathsParams) error {
	if len(params.Files) == 0 {
		return nil
	}

	args := append([]string{"-C", params.WorktreePath, "reset", "--quiet", "HEAD", "--"}, params.Files...)
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git reset: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// CopyPathParams holds inputs for copying a file or directory.
type CopyPathParams struct {
	SourceDir string
	TargetDir string
	RelPath   string
}

// CopyPath copies RelPath from SourceDir to TargetDir, preserving permissions
// and creating parent directories. Directories are copied recursively, which
// covers untracked entries that git reports as a directory.
func CopyPath(params CopyPathParams) error {
	src := filepath.Join(params.SourceDir, params.RelPath)
	dst := filepath.Join(params.TargetDir, params.RelPath)

	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat %s: %w", src, err)
	}

	if info.IsDir() {
		return copyTree(src, dst)
	}
	return copyFile(src, dst, info.Mode())
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create dir for %s: %w", dst, err)
	}
	if err := os.WriteFile(dst, data, mode); err != nil {
		return fmt.Errorf("write %s: %w", dst, err)
	}
	return nil
}
