package dashboard

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/flow"
)

func TestConfirmOptionsNameBothOutcomesWhenLabelled(t *testing.T) {
	options := confirmOptions(flow.ConfirmParams{YesLabel: "Push to origin", NoLabel: "Keep local"})

	if len(options) != 3 {
		t.Fatalf("a labelled confirm must offer both outcomes, got %d options", len(options))
	}
	if options[0].Label != "Keep local" || options[2].Label != "Push to origin" {
		t.Fatalf("the harmless outcome must lead, got %+v", options)
	}
}

func TestConfirmOptionsStayPlainWithoutLabels(t *testing.T) {
	options := confirmOptions(flow.ConfirmParams{})

	if len(options) != 1 || options[0].Value != confirmYes {
		t.Fatalf("an unlabelled confirm must keep its single option, got %+v", options)
	}
}
