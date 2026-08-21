package compose

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type VerifyAllParams struct {
	ProjectDir string
	ByFile     map[string][]domain.ComposePortBinding
}

// VerifyAll reports, per file, why its recorded positions can no longer be
// trusted — the file changed since it was scanned, or it became unreadable. It
// writes nothing, so the caller can withdraw those files from the plan before
// anything is committed to disk rather than discovering it half way through.
func VerifyAll(params VerifyAllParams) map[string]string {
	failures := map[string]string{}
	for _, file := range rules.SortedComposeFiles(params.ByFile) {
		if _, err := renderPatched(params.ProjectDir, file, params.ByFile[file]); err != nil {
			failures[file] = err.Error()
		}
	}
	return failures
}

type PatchAllParams struct {
	ProjectDir string
	ByFile     map[string][]domain.ComposePortBinding
}

// PatchAll rewrites every file's host ports. It renders all of them first and
// only then writes, so a position that no longer matches aborts before any file
// on disk has been touched.
func PatchAll(params PatchAllParams) error {
	files := rules.SortedComposeFiles(params.ByFile)

	rendered := make([]patchedFile, 0, len(files))
	for _, file := range files {
		out, err := renderPatched(params.ProjectDir, file, params.ByFile[file])
		if err != nil {
			return err
		}
		if out.changed {
			rendered = append(rendered, out)
		}
	}

	var written []string
	for _, out := range rendered {
		if err := writeFile(out); err != nil {
			if len(written) == 0 {
				return err
			}
			return fmt.Errorf("%w — %s", err, fmt.Sprintf(domain.ComposePatchPartialFmt, strings.Join(written, ", ")))
		}
		written = append(written, out.file)
	}
	return nil
}

type patchedFile struct {
	file    string
	path    string
	content string
	mode    os.FileMode
	changed bool
}

func renderPatched(projectDir, file string, bindings []domain.ComposePortBinding) (patchedFile, error) {
	path := filepath.Join(projectDir, file)

	info, err := os.Stat(path)
	if err != nil {
		return patchedFile{}, fmt.Errorf(domain.ComposeReadFileFmt, file, err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return patchedFile{}, fmt.Errorf(domain.ComposeReadFileFmt, file, err)
	}

	patched, err := rules.ApplyComposePortPatches(rules.ApplyComposePortPatchesParams{
		Content:  string(content),
		Bindings: bindings,
	})
	if err != nil {
		return patchedFile{}, err
	}

	return patchedFile{
		file:    file,
		path:    path,
		content: patched,
		mode:    info.Mode().Perm(),
		changed: patched != string(content),
	}, nil
}

// writeFile replaces the file through a temporary neighbour: a compose file
// truncated by an interrupted write would take the project's stack down with it.
func writeFile(out patchedFile) error {
	temp, err := os.CreateTemp(filepath.Dir(out.path), filepath.Base(out.path)+".wtm-*")
	if err != nil {
		return fmt.Errorf(domain.ComposeWriteFileFmt, out.file, err)
	}
	tempPath := temp.Name()

	if _, err := temp.WriteString(out.content); err != nil {
		temp.Close()
		os.Remove(tempPath)
		return fmt.Errorf(domain.ComposeWriteFileFmt, out.file, err)
	}
	if err := temp.Close(); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf(domain.ComposeWriteFileFmt, out.file, err)
	}
	if err := os.Chmod(tempPath, out.mode); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf(domain.ComposeWriteFileFmt, out.file, err)
	}
	if err := os.Rename(tempPath, out.path); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf(domain.ComposeWriteFileFmt, out.file, err)
	}
	return nil
}
