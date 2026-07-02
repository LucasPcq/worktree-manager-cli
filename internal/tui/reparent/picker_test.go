package reparent

import (
	"strings"
	"testing"
)

func TestSingleParentDescriptionShowsCurrentParent(t *testing.T) {
	desc := singleParentDescription("dev/a")
	if !strings.Contains(desc, "dev/a") {
		t.Fatalf("description should name the current parent, got %q", desc)
	}
}

func TestSingleParentDescriptionHandlesNoParent(t *testing.T) {
	// A merged-then-cleaned parent leaves no recorded value; the description must
	// still render (no badge that would silently vanish).
	desc := singleParentDescription("")
	if desc == "" || strings.Contains(desc, "Currently rebased onto") {
		t.Fatalf("empty parent should yield a distinct no-parent description, got %q", desc)
	}
}

func TestParentStepDescriptionBatchNamesCount(t *testing.T) {
	desc := parentStepDescription([]string{"a", "b", "c"}, nil)
	if !strings.Contains(desc, "3") {
		t.Fatalf("batch description should name the selection count, got %q", desc)
	}
}

func TestParentStepDescriptionSingleFallsBackToRecordedParent(t *testing.T) {
	desc := parentStepDescription([]string{"feat"}, map[string]string{"feat": "main"})
	if !strings.Contains(desc, "main") {
		t.Fatalf("single-selection description should name the current parent, got %q", desc)
	}
}
