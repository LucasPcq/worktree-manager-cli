package commands

import "testing"

func TestJoinServiceNames_Empty(t *testing.T) {
	if got := joinServiceNames(nil); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestJoinServiceNames_One(t *testing.T) {
	got := joinServiceNames([]string{"api"})
	if got != "api" {
		t.Errorf("got %q, want %q", got, "api")
	}
}

func TestJoinServiceNames_Multiple(t *testing.T) {
	got := joinServiceNames([]string{"api", "web", "worker"})
	want := "api, web, worker"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
