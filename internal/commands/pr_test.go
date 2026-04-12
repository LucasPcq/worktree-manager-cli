package commands

import "testing"

func TestTruncate_ShorterThanMax(t *testing.T) {
	got := truncate("hello", 10)
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestTruncate_EqualToMax(t *testing.T) {
	got := truncate("hello", 5)
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestTruncate_LongerThanMax(t *testing.T) {
	got := truncate("hello world", 6)
	want := "hello…"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTruncate_Empty(t *testing.T) {
	got := truncate("", 10)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}
