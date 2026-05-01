// Package runconfig orchestrates load + validate + write on the run.toml
// config file. Mirrors the layering of internal/service/worktree: pure I/O
// over config/ + rules/, no cobra/bubbletea.
package runconfig

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// Load reads run.toml from the state directory. Returns an empty RunConfig
// (no error) when the file is absent — callers can mutate-and-Save to
// initialise the file.
func Load(stateDir string) (domain.RunConfig, error) {
	return config.LoadRun(stateDir)
}

// SaveParams holds the inputs for Save.
type SaveParams struct {
	StateDir string
	Config   domain.RunConfig
}

// Save validates cfg via rules.ValidateRun and writes it to <stateDir>/run.toml,
// overwriting any previous content. Validation errors are joined with "; "
// and prefixed "invalid run config: ".
func Save(params SaveParams) error {
	if _, errs := rules.ValidateRun(params.Config); len(errs) > 0 {
		return fmt.Errorf("invalid run config: %s", strings.Join(errs, "; "))
	}
	return config.WriteRun(config.WriteRunParams{
		StateDir: params.StateDir,
		Config:   params.Config,
		Force:    true,
	})
}
