package github

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestConvertGHPRAggregatesStatusCheckRollup(t *testing.T) {
	g := ghPR{
		Number:         67,
		ReviewDecision: domain.GHReviewDecisionChangesRequested,
		StatusCheckRollup: []ghCheckRun{
			{Conclusion: domain.GHCheckConclusionSuccess},
			{Conclusion: domain.GHCheckConclusionSuccess},
			{Conclusion: domain.GHCheckConclusionNeutral},
			{Conclusion: domain.GHCheckConclusionSkipped},
			{Conclusion: domain.GHCheckConclusionFailure},
			{Conclusion: domain.GHCheckConclusionCancelled},
			{Conclusion: domain.GHCheckConclusionTimedOut},
			{Conclusion: domain.GHCheckConclusionActionRequired},
			{Status: "QUEUED"},
			{Status: "IN_PROGRESS"},
		},
	}

	pr := convertGHPR(g)

	want := domain.PRChecks{Passed: 4, Failed: 4, Pending: 2}
	if pr.Checks != want {
		t.Errorf("Checks = %+v, want %+v", pr.Checks, want)
	}
	if pr.ReviewDecision != domain.GHReviewDecisionChangesRequested {
		t.Errorf("ReviewDecision = %q, want %q", pr.ReviewDecision, domain.GHReviewDecisionChangesRequested)
	}
}

func TestConvertGHPRNoChecksLeavesZeroValue(t *testing.T) {
	pr := convertGHPR(ghPR{Number: 68})

	want := domain.PRChecks{}
	if pr.Checks != want {
		t.Errorf("Checks = %+v, want zero value %+v", pr.Checks, want)
	}
	if pr.ReviewDecision != "" {
		t.Errorf("ReviewDecision = %q, want empty", pr.ReviewDecision)
	}
}
