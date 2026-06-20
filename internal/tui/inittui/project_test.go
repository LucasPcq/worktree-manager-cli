package inittui

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

func TestMoveToFront(t *testing.T) {
	items := []components.SelectItem{
		{Value: "example"},
		{Value: "main"},
		{Value: "parent"},
	}

	got := moveToFront(items, "parent")
	if got[0].Value != "parent" {
		t.Errorf("expected parent first, got %q", got[0].Value)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 items, got %d", len(got))
	}

	// Unknown / empty value leaves order untouched.
	if moveToFront(items, "")[0].Value != "example" {
		t.Error("empty value should not reorder")
	}
	if moveToFront(items, "nope")[0].Value != "example" {
		t.Error("unknown value should not reorder")
	}
}

func TestPrefillSelected(t *testing.T) {
	// Full init (nil prefill) uses the detection default.
	if !prefillSelected(nil, false, true) {
		t.Error("nil prefill should use the full-init default (true)")
	}
	if prefillSelected(nil, true, false) {
		t.Error("nil prefill should use the full-init default (false)")
	}

	// Re-init checks items already in the current config.
	if !prefillSelected(&SectionPrefill{}, true, false) {
		t.Error("configured item should be checked")
	}
	if prefillSelected(&SectionPrefill{}, false, true) {
		t.Error("non-configured item should be unchecked regardless of detection default")
	}
}

func TestFormatOnCreate(t *testing.T) {
	if got := formatOnCreate(nil); got != "" {
		t.Errorf("empty hooks should yield empty string, got %q", got)
	}
	hooks := []domain.HookCommand{
		{Cmd: "pnpm install"},
		{Cmd: "pnpm install", Cwd: "packages/api"},
	}
	want := "Current: pnpm install · pnpm install @packages/api"
	if got := formatOnCreate(hooks); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
