package detect

import "github.com/LucasPcq/wtm/internal/infra"

// BaseBranch returns the default remote branch for the repo at projectDir.
func BaseBranch(projectDir string) string {
	return infra.BaseBranch(projectDir)
}
