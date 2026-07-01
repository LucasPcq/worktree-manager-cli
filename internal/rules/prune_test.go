package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func status(branch string, opts func(*domain.WorktreeStatus)) domain.WorktreeStatus {
	s := domain.WorktreeStatus{Branch: branch, Path: "/wt/" + branch}
	if opts != nil {
		opts(&s)
	}
	return s
}

func node(branch, parent string) domain.WorktreeNode {
	return domain.WorktreeNode{Branch: branch, Path: "/wt/" + branch, SourceBranch: parent}
}

func selectedBranches(plan domain.PrunePlan) map[string]string {
	m := make(map[string]string)
	for _, c := range plan.Selected {
		m[c.Branch] = c.Reason
	}
	return m
}

func skippedBranches(plan domain.PrunePlan) map[string]string {
	m := make(map[string]string)
	for _, s := range plan.Skipped {
		m[s.Branch] = s.Reason
	}
	return m
}

func TestClassifyPruneMerged(t *testing.T) {
	plan := ClassifyPrune(ClassifyPruneParams{
		Statuses: []domain.WorktreeStatus{
			status("done", func(s *domain.WorktreeStatus) { s.CommitsAhead = 0 }),
			status("wip", func(s *domain.WorktreeStatus) { s.CommitsAhead = 3 }),
		},
		Nodes:      []domain.WorktreeNode{node("done", "main"), node("wip", "main")},
		Merged:     true,
		BaseBranch: "main",
	})

	sel := selectedBranches(plan)
	if sel["done"] != domain.PruneReasonMerged {
		t.Errorf("expected 'done' selected as merged, got %v", sel)
	}
	if _, ok := sel["wip"]; ok {
		t.Errorf("'wip' should not be selected (3 commits ahead)")
	}
}

func TestClassifyPrunePRState(t *testing.T) {
	plan := ClassifyPrune(ClassifyPruneParams{
		Statuses: []domain.WorktreeStatus{
			status("merged-pr", func(s *domain.WorktreeStatus) { s.CommitsAhead = 5 }),
			status("closed-pr", func(s *domain.WorktreeStatus) { s.CommitsAhead = 5 }),
			status("open-pr", func(s *domain.WorktreeStatus) { s.CommitsAhead = 5 }),
		},
		Nodes: []domain.WorktreeNode{node("merged-pr", "main"), node("closed-pr", "main"), node("open-pr", "main")},
		PRStates: map[string]string{
			"merged-pr": domain.PRStateMerged,
			"closed-pr": domain.PRStateClosed,
			"open-pr":   domain.PRStateOpen,
		},
		Closed:     true,
		BaseBranch: "main",
	})

	sel := selectedBranches(plan)
	if sel["merged-pr"] != domain.PruneReasonPRMerged {
		t.Errorf("merged-pr reason = %q, want %q", sel["merged-pr"], domain.PruneReasonPRMerged)
	}
	if sel["closed-pr"] != domain.PruneReasonPRClosed {
		t.Errorf("closed-pr reason = %q, want %q", sel["closed-pr"], domain.PruneReasonPRClosed)
	}
	if _, ok := sel["open-pr"]; ok {
		t.Errorf("open-pr should not be selected")
	}
}

func TestClassifyPruneGone(t *testing.T) {
	plan := ClassifyPrune(ClassifyPruneParams{
		Statuses:   []domain.WorktreeStatus{status("gone", func(s *domain.WorktreeStatus) { s.CommitsAhead = 4 })},
		Nodes:      []domain.WorktreeNode{node("gone", "main")},
		Gone:       map[string]bool{"gone": true},
		GoneFilter: true,
		BaseBranch: "main",
	})
	if selectedBranches(plan)["gone"] != domain.PruneReasonGone {
		t.Errorf("expected 'gone' selected via gone filter, got %v", plan.Selected)
	}
}

func TestClassifyPruneDirtySkipUnlessForce(t *testing.T) {
	build := func(force bool) domain.PrunePlan {
		return ClassifyPrune(ClassifyPruneParams{
			Statuses:   []domain.WorktreeStatus{status("dirty", func(s *domain.WorktreeStatus) { s.CommitsAhead = 0; s.IsDirty = true })},
			Nodes:      []domain.WorktreeNode{node("dirty", "main")},
			Merged:     true,
			BaseBranch: "main",
			Force:      force,
		})
	}

	skip := build(false)
	if skippedBranches(skip)["dirty"] != domain.PruneSkipDirty {
		t.Errorf("without force, dirty should be skipped, got %+v", skip)
	}
	if len(skip.Selected) != 0 {
		t.Errorf("without force, dirty should not be selected")
	}

	forced := build(true)
	if selectedBranches(forced)["dirty"] != domain.PruneReasonMerged {
		t.Errorf("with force, dirty should be selected, got %+v", forced)
	}
}

func TestClassifyPruneUnpushedSkipUnlessForce(t *testing.T) {
	// A gone branch with local commits not on the remote is unsafe: without force
	// it is skipped (guarding against silent loss of committed work); with force it
	// is selected but tagged so the confirm step can re-gate it.
	build := func(force bool) domain.PrunePlan {
		return ClassifyPrune(ClassifyPruneParams{
			Statuses:   []domain.WorktreeStatus{status("gone", func(s *domain.WorktreeStatus) { s.CommitsAhead = 2 })},
			Nodes:      []domain.WorktreeNode{node("gone", "main")},
			Gone:       map[string]bool{"gone": true},
			Unpushed:   map[string]int{"gone": 2},
			GoneFilter: true,
			BaseBranch: "main",
			Force:      force,
		})
	}

	skip := build(false)
	if skippedBranches(skip)["gone"] != domain.PruneSkipUnpushed {
		t.Errorf("without force, unpushed gone branch should be skipped as unpushed, got %+v", skip)
	}
	if len(skip.Selected) != 0 {
		t.Errorf("without force, unpushed branch should not be selected")
	}

	forced := build(true)
	if len(forced.Selected) != 1 || forced.Selected[0].UnsafeReason != domain.PruneSkipUnpushed {
		t.Errorf("with force, unpushed branch should be selected and tagged unpushed, got %+v", forced.Selected)
	}
}

func TestClassifyPruneOpenPRSkipUnlessForce(t *testing.T) {
	// A gone branch that still has an open PR is unsafe without force (mirrors
	// clean refusing to remove a worktree with an open PR).
	build := func(force bool) domain.PrunePlan {
		return ClassifyPrune(ClassifyPruneParams{
			Statuses:   []domain.WorktreeStatus{status("gone", func(s *domain.WorktreeStatus) { s.CommitsAhead = 0 })},
			Nodes:      []domain.WorktreeNode{node("gone", "main")},
			Gone:       map[string]bool{"gone": true},
			PRStates:   map[string]string{"gone": domain.PRStateOpen},
			GoneFilter: true,
			BaseBranch: "main",
			Force:      force,
		})
	}

	skip := build(false)
	if skippedBranches(skip)["gone"] != domain.PruneSkipOpenPR {
		t.Errorf("without force, open-PR branch should be skipped as open_pr, got %+v", skip)
	}

	forced := build(true)
	if len(forced.Selected) != 1 || forced.Selected[0].UnsafeReason != domain.PruneSkipOpenPR {
		t.Errorf("with force, open-PR branch should be selected and tagged open_pr, got %+v", forced.Selected)
	}
}

func TestClassifyPruneProtections(t *testing.T) {
	// The base branch is protected; the current worktree is NOT (prune removes it
	// like clean), so a merged non-base worktree is selected.
	plan := ClassifyPrune(ClassifyPruneParams{
		Statuses: []domain.WorktreeStatus{
			status("main", func(s *domain.WorktreeStatus) { s.CommitsAhead = 0; s.IsParent = true }),
			status("current", func(s *domain.WorktreeStatus) { s.CommitsAhead = 0 }),
		},
		Nodes:      []domain.WorktreeNode{{Branch: "main", IsMain: true}, node("current", "main")},
		Merged:     true,
		BaseBranch: "main",
	})
	// main is both the base branch and the main worktree; base takes precedence.
	if skippedBranches(plan)["main"] != domain.PruneSkipBase {
		t.Errorf("main should be skipped as base_branch, got %v", skippedBranches(plan))
	}
	if selectedBranches(plan)["current"] != domain.PruneReasonMerged {
		t.Errorf("current worktree should be selectable (not protected), got %+v", plan.Selected)
	}
}

func TestClassifyPruneSkipsMainWorktree(t *testing.T) {
	// A main worktree whose branch differs from the configured base is still
	// protected as the main worktree.
	plan := ClassifyPrune(ClassifyPruneParams{
		Statuses:   []domain.WorktreeStatus{status("trunk", func(s *domain.WorktreeStatus) { s.CommitsAhead = 0; s.IsParent = true })},
		Nodes:      []domain.WorktreeNode{{Branch: "trunk", IsMain: true}},
		Merged:     true,
		BaseBranch: "main",
	})
	if skippedBranches(plan)["trunk"] != domain.PruneSkipMain {
		t.Errorf("trunk should be skipped as main_worktree, got %+v", plan.Skipped)
	}
}

func TestFinalizePrunePlanReparentsDeselectedChild(t *testing.T) {
	// Full plan prunes A and its child B (both merged); C survives and moves to
	// main. The user deselects B (keeps it) — B must now reparent onto main too.
	full := domain.PrunePlan{
		Selected: []domain.PruneCandidate{
			{Branch: "A", SourceBranch: "main", Reason: domain.PruneReasonMerged},
			{Branch: "B", SourceBranch: "A", Reason: domain.PruneReasonMerged},
		},
		Reparents: []domain.ReparentResult{{Branch: "C", OldParent: "A", NewParent: "main"}},
	}

	out := FinalizePrunePlan(FinalizePrunePlanParams{Plan: full, Chosen: []string{"A"}, BaseBranch: "main"})

	if len(out.Selected) != 1 || out.Selected[0].Branch != "A" {
		t.Fatalf("expected only A selected, got %+v", out.Selected)
	}
	got := make(map[string]string)
	for _, r := range out.Reparents {
		got[r.Branch] = r.NewParent
	}
	if got["C"] != "main" {
		t.Errorf("C should reparent onto main, got %v", got)
	}
	if got["B"] != "main" {
		t.Errorf("deselected B should reparent onto main, got %v", got)
	}
}

func TestFinalizePrunePlanDropsUnsafeWithoutForce(t *testing.T) {
	// Each unsafe candidate is dropped to Skipped with the reason that made it
	// unsafe (dirty / unpushed / open PR), unless the user confirms with force.
	full := domain.PrunePlan{
		Selected: []domain.PruneCandidate{
			{Branch: "clean-wt", SourceBranch: "main", Reason: domain.PruneReasonMerged},
			{Branch: "dirty-wt", SourceBranch: "main", Reason: domain.PruneReasonMerged, IsDirty: true, UnsafeReason: domain.PruneSkipDirty},
			{Branch: "unpushed-wt", SourceBranch: "main", Reason: domain.PruneReasonGone, UnsafeReason: domain.PruneSkipUnpushed},
			{Branch: "openpr-wt", SourceBranch: "main", Reason: domain.PruneReasonGone, UnsafeReason: domain.PruneSkipOpenPR},
		},
	}
	chosen := []string{"clean-wt", "dirty-wt", "unpushed-wt", "openpr-wt"}

	// Without force: every unsafe candidate is dropped to skipped with its reason.
	noForce := FinalizePrunePlan(FinalizePrunePlanParams{Plan: full, Chosen: chosen, BaseBranch: "main", Force: false})
	if len(noForce.Selected) != 1 || noForce.Selected[0].Branch != "clean-wt" {
		t.Errorf("without force, only clean-wt should be selected, got %+v", noForce.Selected)
	}
	skipped := skippedBranches(noForce)
	if skipped["dirty-wt"] != domain.PruneSkipDirty {
		t.Errorf("dirty-wt should be skipped as dirty, got %+v", noForce.Skipped)
	}
	if skipped["unpushed-wt"] != domain.PruneSkipUnpushed {
		t.Errorf("unpushed-wt should be skipped as unpushed, got %+v", noForce.Skipped)
	}
	if skipped["openpr-wt"] != domain.PruneSkipOpenPR {
		t.Errorf("openpr-wt should be skipped as open_pr, got %+v", noForce.Skipped)
	}

	// With force: all unsafe candidates are kept.
	forced := FinalizePrunePlan(FinalizePrunePlanParams{Plan: full, Chosen: chosen, BaseBranch: "main", Force: true})
	if len(forced.Selected) != 4 {
		t.Errorf("with force, all four should be selected, got %+v", forced.Selected)
	}
}

func TestClassifyPruneReparents(t *testing.T) {
	// feat is pruned; its surviving child moves to feat's parent (main). A child
	// that is also pruned is not reparented.
	plan := ClassifyPrune(ClassifyPruneParams{
		Statuses: []domain.WorktreeStatus{
			status("feat", func(s *domain.WorktreeStatus) { s.CommitsAhead = 0 }),
			status("child-live", func(s *domain.WorktreeStatus) { s.CommitsAhead = 2 }),
			status("child-dead", func(s *domain.WorktreeStatus) { s.CommitsAhead = 0 }),
		},
		Nodes: []domain.WorktreeNode{
			node("feat", "main"),
			node("child-live", "feat"),
			node("child-dead", "feat"),
		},
		Merged:     true,
		BaseBranch: "main",
	})

	if len(plan.Reparents) != 1 {
		t.Fatalf("expected 1 reparent, got %+v", plan.Reparents)
	}
	r := plan.Reparents[0]
	if r.Branch != "child-live" || r.OldParent != "feat" || r.NewParent != "main" {
		t.Errorf("unexpected reparent: %+v", r)
	}
}
