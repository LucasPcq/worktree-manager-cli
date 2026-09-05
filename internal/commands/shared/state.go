package shared

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
)

// EnvStateDir overrides the resolved state directory; useful for tests and CI.
const EnvStateDir = "WTM_STATE_DIR"

// StateDir returns <git-common-dir>/wtm/, the wtm state root for the current
// clone. WTM_STATE_DIR overrides git resolution.
func StateDir(dir string) (string, error) {
	if override := os.Getenv(EnvStateDir); override != "" {
		return override, nil
	}
	commonDir, err := infra.GitCommonDir(infra.GitCommonDirParams{Dir: dir})
	if err != nil {
		return "", fmt.Errorf("resolve state dir: %w", err)
	}
	return filepath.Join(commonDir, domain.StateDirName), nil
}
