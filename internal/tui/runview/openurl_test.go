package runview

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/flow/runlogs"
)

func TestOpenKeyDoesNothingWithoutAURL(t *testing.T) {
	opened := ""
	model := New(Params{Open: func(url string) error {
		opened = url
		return nil
	}})
	model.selected = "web"

	if cmd := model.openSelectedURL(); cmd != nil {
		t.Fatalf("a job that publishes no URL must produce no command")
	}
	if opened != "" {
		t.Errorf("nothing must have been opened, got %q", opened)
	}
}

func TestOpenKeyOpensTheSelectedJobURL(t *testing.T) {
	opened := ""
	model := New(Params{Open: func(url string) error {
		opened = url
		return nil
	}})
	model.selected = "web"
	model.sequence.remember(runlogs.Event{Job: "web", URL: "http://localhost:3010"})

	cmd := model.openSelectedURL()
	if cmd == nil {
		t.Fatal("a published job must produce a command")
	}
	cmd()

	if opened != "http://localhost:3010" {
		t.Errorf("opened %q, want http://localhost:3010", opened)
	}
}
