package flow

import "testing"

func TestConfirmDescriptionFoldsTheWarningIn(t *testing.T) {
	got := ConfirmDescription(ConfirmParams{Description: "review this", Warning: "cannot be undone"})
	if want := "review this\ncannot be undone"; got != want {
		t.Errorf("description = %q, want %q", got, want)
	}
}

func TestConfirmDescriptionWithoutAWarningStaysThePlainDescription(t *testing.T) {
	if got := ConfirmDescription(ConfirmParams{Description: "review this"}); got != "review this" {
		t.Errorf("description = %q, want the description unchanged", got)
	}
}

func TestConfirmDescriptionWithOnlyAWarningIsTheWarning(t *testing.T) {
	if got := ConfirmDescription(ConfirmParams{Warning: "cannot be undone"}); got != "cannot be undone" {
		t.Errorf("description = %q, want the warning alone", got)
	}
}
