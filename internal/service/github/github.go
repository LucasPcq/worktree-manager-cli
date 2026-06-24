package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

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
	Body              string       `json:"body"`
	Author            ghAuthor     `json:"author"`
	HeadRefName       string       `json:"headRefName"`
	BaseRefName       string       `json:"baseRefName"`
	IsDraft           bool         `json:"isDraft"`
	CreatedAt         time.Time    `json:"createdAt"`
	URL               string       `json:"url"`
	IsCrossRepository bool         `json:"isCrossRepository"`
	StatusCheckRollup []ghCheckRun `json:"statusCheckRollup"`
	Reviews           []ghReview   `json:"reviews"`
}

type ghAuthor struct {
	Login string `json:"login"`
}

type ghCheckRun struct {
	Conclusion string `json:"conclusion"`
	Status     string `json:"status"`
}

type ghReview struct {
	Author ghAuthor `json:"author"`
	State  string   `json:"state"`
}

func convertGHPR(g ghPR) domain.PRInfo {
	return domain.PRInfo{
		Number:     g.Number,
		Title:      g.Title,
		Body:       g.Body,
		Author:     g.Author.Login,
		Branch:     g.HeadRefName,
		BaseBranch: g.BaseRefName,
		State:      "open",
		Draft:      g.IsDraft,
		CreatedAt:  g.CreatedAt,
		URL:        g.URL,
		IsFork:     g.IsCrossRepository,
		CIStatus:   deriveCIStatus(g.StatusCheckRollup),
		Reviews:    deduplicateReviews(g.Reviews),
	}
}

func deriveCIStatus(checks []ghCheckRun) domain.CIStatus {
	if len(checks) == 0 {
		return domain.CIStatusNone
	}

	hasPending := false
	for _, c := range checks {
		switch c.Conclusion {
		case "failure", "timed_out", "cancelled":
			return domain.CIStatusFailing
		case "":
			hasPending = true
		}
	}

	if hasPending {
		return domain.CIStatusPending
	}
	return domain.CIStatusPassing
}

func deduplicateReviews(reviews []ghReview) []domain.ReviewInfo {
	latest := make(map[string]domain.ReviewInfo)
	for _, r := range reviews {
		latest[r.Author.Login] = domain.ReviewInfo{
			User:  r.Author.Login,
			State: r.State,
		}
	}

	result := make([]domain.ReviewInfo, 0, len(latest))
	for _, r := range latest {
		result = append(result, r)
	}
	return result
}

func parseJSON[T any](data []byte) (T, error) {
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return v, fmt.Errorf("parse gh JSON: %w", err)
	}
	return v, nil
}
