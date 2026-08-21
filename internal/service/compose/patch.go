package compose

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type PatchParams struct {
	ProjectDir string
	File       string
	Bindings   []domain.ComposePortBinding
}

// Patch rewrites the host port of each binding in the file it came from. The
// content is re-read rather than carried over from the scan, so a file edited
// in between fails the position check instead of being overwritten.
func Patch(params PatchParams) error {
	path := filepath.Join(params.ProjectDir, params.File)

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", params.File, err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", params.File, err)
	}

	patched, err := rules.ApplyComposePortPatches(rules.ApplyComposePortPatchesParams{
		Content:  string(content),
		Bindings: params.Bindings,
	})
	if err != nil {
		return err
	}
	if patched == string(content) {
		return nil
	}

	if err := os.WriteFile(path, []byte(patched), info.Mode().Perm()); err != nil {
		return fmt.Errorf("write %s: %w", params.File, err)
	}
	return nil
}

// PatchAll applies every file's bindings, stopping at the first failure. A
// partial rewrite is left in place: each file is patched atomically on its own,
// and the error names the one that failed.
func PatchAll(projectDir string, byFile map[string][]domain.ComposePortBinding) error {
	for _, file := range rules.SortedComposeFiles(byFile) {
		if err := Patch(PatchParams{ProjectDir: projectDir, File: file, Bindings: byFile[file]}); err != nil {
			return err
		}
	}
	return nil
}
