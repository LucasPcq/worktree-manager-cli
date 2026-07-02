package reparent

import (
	"strings"
	"testing"
)

func TestParentStepDescriptionShowsCurrentParent(t *testing.T) {
	desc := parentStepDescription("dev/a")
	if !strings.Contains(desc, "dev/a") {
		t.Fatalf("description should name the current parent, got %q", desc)
	}
}

func TestParentStepDescriptionHandlesNoParent(t *testing.T) {
	// A merged-then-cleaned parent leaves no recorded value; the description must
	// still render (no badge that would silently vanish).
	desc := parentStepDescription("")
	if desc == "" || strings.Contains(desc, "Currently rebased onto") {
		t.Fatalf("empty parent should yield a distinct no-parent description, got %q", desc)
	}
}
