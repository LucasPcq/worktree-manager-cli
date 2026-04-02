package detect

import (
	"os/exec"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// BaseBranch detects the default branch via git symbolic-ref.
// Returns domain.DefaultBaseBranch if detection fails.
func BaseBranch(projectDir string) string {
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = projectDir
	out, err := cmd.Output()
	if err != nil {
		return domain.DefaultBaseBranch
	}

	ref := strings.TrimSpace(string(out))
	parts := strings.Split(ref, "/")
	if len(parts) == 0 {
		return domain.DefaultBaseBranch
	}

	return parts[len(parts)-1]
}
