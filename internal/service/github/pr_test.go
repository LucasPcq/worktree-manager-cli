package github

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// TestConvertGHPRAggregatesStatusCheckRollup pins the three-bucket mapping of
// a modern CheckRun's `conclusion`: SUCCESS/NEUTRAL/SKIPPED pass, ACTION_REQUIRED
// is not-yet-run (pending, never failed — it has not run and broken, it has
// not started), FAILURE/CANCELLED/TIMED_OUT fail, and no conclusion at all
// (still running) is pending.
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

	want := domain.PRChecks{Passed: 4, Failed: 3, Pending: 3}
	if pr.Checks != want {
		t.Errorf("Checks = %+v, want %+v", pr.Checks, want)
	}
	if pr.ReviewDecision != domain.GHReviewDecisionChangesRequested {
		t.Errorf("ReviewDecision = %q, want %q", pr.ReviewDecision, domain.GHReviewDecisionChangesRequested)
	}
}

// TestConvertGHPRDecodesLegacyStatusContext pins the critical fix: a
// StatusContext entry (CircleCI, Travis, any integration on the legacy Status
// API rather than the Checks API) reports through `state`, not `conclusion`.
// Without decoding State, every such entry has an empty Conclusion and is
// silently counted as pending forever, however the check actually resolved.
func TestConvertGHPRDecodesLegacyStatusContext(t *testing.T) {
	g := ghPR{
		Number: 67,
		StatusCheckRollup: []ghCheckRun{
			{State: domain.GHCheckStateSuccess},
			{State: domain.GHCheckStateError},
			{State: domain.GHCheckStateFailure},
			{State: domain.GHCheckStatePending},
		},
	}

	pr := convertGHPR(g)

	want := domain.PRChecks{Passed: 1, Failed: 2, Pending: 1}
	if pr.Checks != want {
		t.Errorf("Checks = %+v, want %+v — a legacy status-context check must resolve to a real outcome, not stay pending forever", pr.Checks, want)
	}
}

// TestConvertGHPRUnknownConclusionFailsClosed pins that an unrecognised
// conclusion is neither counted as a success nor as still-running.
func TestConvertGHPRUnknownConclusionFailsClosed(t *testing.T) {
	g := ghPR{
		Number:            67,
		StatusCheckRollup: []ghCheckRun{{Conclusion: "SOME_FUTURE_CONCLUSION"}},
	}

	pr := convertGHPR(g)

	want := domain.PRChecks{Failed: 1}
	if pr.Checks != want {
		t.Errorf("Checks = %+v, want %+v — an unrecognised conclusion must fail closed", pr.Checks, want)
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
