package run

import "testing"

func TestJoinJobNames_Empty(t *testing.T) {
	if got := joinJobNames(nil); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestJoinJobNames_One(t *testing.T) {
	got := joinJobNames([]string{"api"})
	if got != "api" {
		t.Errorf("got %q, want %q", got, "api")
	}
}

func TestJoinJobNames_Multiple(t *testing.T) {
	got := joinJobNames([]string{"api", "web", "worker"})
	want := "api, web, worker"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
