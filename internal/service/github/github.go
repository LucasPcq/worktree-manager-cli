package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/LucasPcq/wtm/internal/domain"
)

func ensureGH() error {
	_, err := exec.LookPath("gh")
	if err != nil {
		return domain.ErrGHNotInstalled
	}
	return nil
}

func ensureAuth() error {
	if err := ensureGH(); err != nil {
		return err
	}
	cmd := exec.Command("gh", "auth", "status")
	if err := cmd.Run(); err != nil {
		return domain.ErrGHNotAuthenticated
	}
	return nil
}

func runGH(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gh %v: %w\n%s", args, err, stderr.String())
	}
	return stdout.Bytes(), nil
}

type ghPR struct {
	Number            int          `json:"number"`
	Title             string       `json:"title"`
	Author            ghAuthor     `json:"author"`
	HeadRefName       string       `json:"headRefName"`
	BaseRefName       string       `json:"baseRefName"`
	URL               string       `json:"url"`
	IsCrossRepository bool         `json:"isCrossRepository"`
	IsDraft           bool         `json:"isDraft"`
	ReviewDecision    string       `json:"reviewDecision"`
	StatusCheckRollup []ghCheckRun `json:"statusCheckRollup"`
}

type ghAuthor struct {
	Login string `json:"login"`
}

// ghCheckRun is one entry of a PR's statusCheckRollup: a finished run carries
// Conclusion, a still-running one leaves it empty and carries Status instead.
type ghCheckRun struct {
	Conclusion string `json:"conclusion"`
	Status     string `json:"status"`
}

func convertGHPR(g ghPR) domain.PRInfo {
	return domain.PRInfo{
		Number:         g.Number,
		Title:          g.Title,
		Author:         g.Author.Login,
		Branch:         g.HeadRefName,
		BaseBranch:     g.BaseRefName,
		State:          domain.PRStateOpen,
		URL:            g.URL,
		IsFork:         g.IsCrossRepository,
		Draft:          g.IsDraft,
		Checks:         aggregateChecks(g.StatusCheckRollup),
		ReviewDecision: g.ReviewDecision,
	}
}

// aggregateChecks buckets a status-check rollup into passed/failed/pending. A
// SKIPPED or NEUTRAL run did not fail, so it counts as passed; CANCELLED,
// TIMED_OUT and ACTION_REQUIRED all block the PR the same way FAILURE does.
// An empty Conclusion means the run is still queued or in progress.
func aggregateChecks(rollup []ghCheckRun) domain.PRChecks {
	var checks domain.PRChecks
	for _, run := range rollup {
		switch run.Conclusion {
		case "":
			checks.Pending++
		case domain.GHCheckConclusionSuccess, domain.GHCheckConclusionNeutral, domain.GHCheckConclusionSkipped:
			checks.Passed++
		default:
			checks.Failed++
		}
	}
	return checks
}

func parseJSON[T any](data []byte) (T, error) {
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return v, fmt.Errorf("parse gh JSON: %w", err)
	}
	return v, nil
}
