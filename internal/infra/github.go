package infra

import (
	"encoding/json"
	"os/exec"
)

// HasOpenPRParams holds inputs for checking open PRs.
type HasOpenPRParams struct {
	ProjectDir string
	Branch     string
}

// HasOpenPR checks if a branch has an open PR via the gh CLI.
// Returns false gracefully if gh is not installed or fails.
func HasOpenPR(params HasOpenPRParams) (bool, string) {
	ghPath, err := exec.LookPath("gh")
	if err != nil {
		return false, ""
	}

	cmd := exec.Command(ghPath, "pr", "view", params.Branch, "--json", "url,state")
	cmd.Dir = params.ProjectDir
	out, err := cmd.Output()
	if err != nil {
		return false, ""
	}

	var result struct {
		URL   string `json:"url"`
		State string `json:"state"`
	}
	if json.Unmarshal(out, &result) != nil {
		return false, ""
	}

	if result.State == "OPEN" {
		return true, result.URL
	}
	return false, ""
}
