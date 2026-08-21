package detect

import (
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type EnvPortCandidatesParams struct {
	// ProjectDir is the main worktree, whose .env files hold the base ports —
	// any other worktree's copy already carries an offset.
	ProjectDir string
	Files      []domain.EnvFile
	Bases      map[string]int
	Existing   []domain.EnvPortLink
}

// EnvPortCandidates reads the project's configured env value files and reports
// the keys whose value holds a declared base port. A file that cannot be read is
// skipped: the detection is a convenience, and refusing `wtm run init` over an
// unreadable .env would be out of proportion.
func EnvPortCandidates(params EnvPortCandidatesParams) []domain.EnvPortLink {
	if len(params.Bases) == 0 {
		return nil
	}

	lines := map[string][]domain.EnvLine{}
	for _, file := range params.Files {
		data, err := os.ReadFile(filepath.Join(params.ProjectDir, file.Target))
		if err != nil {
			continue
		}
		lines[file.Target] = rules.ParseEnv(string(data))
	}

	return rules.EnvPortCandidates(rules.EnvPortCandidatesParams{
		Lines:    lines,
		Bases:    params.Bases,
		Existing: params.Existing,
	})
}
