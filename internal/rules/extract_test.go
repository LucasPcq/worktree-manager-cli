package rules_test

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestClassifyExtractStatus(t *testing.T) {
	cases := []struct {
		status string
		want   domain.ExtractFileStatus
	}{
		{"??", domain.ExtractStatusUntracked},
		{"M", domain.ExtractStatusModified},
		{"MM", domain.ExtractStatusModified},
		{"A", domain.ExtractStatusModified},
		{"D", domain.ExtractStatusDeleted},
		{"AD", domain.ExtractStatusDeleted},
		// Raw, untrimmed XY fields as `git status --porcelain -z` emits them.
		{" M", domain.ExtractStatusModified},
		{"M ", domain.ExtractStatusModified},
		{" D", domain.ExtractStatusDeleted},
		{"D ", domain.ExtractStatusDeleted},
		{"A ", domain.ExtractStatusModified},
		// Renames win over the deletion they also carry.
		{"R ", domain.ExtractStatusRenamed},
		{"RM", domain.ExtractStatusRenamed},
		{"RD", domain.ExtractStatusRenamed},
		{"C ", domain.ExtractStatusRenamed},
	}

	for _, c := range cases {
		if got := rules.ClassifyExtractStatus(c.status); got != c.want {
			t.Errorf("ClassifyExtractStatus(%q) = %q, want %q", c.status, got, c.want)
		}
	}
}

func TestExtractStatusLabel(t *testing.T) {
	cases := map[domain.ExtractFileStatus]string{
		domain.ExtractStatusUntracked: "new",
		domain.ExtractStatusDeleted:   "del",
		domain.ExtractStatusModified:  "mod",
		domain.ExtractStatusRenamed:   "ren",
	}
	for status, want := range cases {
		if got := rules.ExtractStatusLabel(status); got != want {
			t.Errorf("ExtractStatusLabel(%q) = %q, want %q", status, got, want)
		}
		if len(want) != 3 {
			t.Errorf("label %q must be 3 chars wide to keep the picker aligned", want)
		}
	}
}

func TestSelectExtractFilesExactPaths(t *testing.T) {
	available := []domain.ExtractFile{
		{Path: "a.txt", Status: domain.ExtractStatusModified},
		{Path: "b.txt", Status: domain.ExtractStatusUntracked},
	}

	got, err := rules.SelectExtractFiles(rules.SelectExtractFilesParams{
		Available: available,
		Paths:     []string{"b.txt"},
	})
	if err != nil {
		t.Fatalf("SelectExtractFiles: %v", err)
	}
	if len(got) != 1 || got[0].Path != "b.txt" {
		t.Fatalf("got %+v, want just b.txt", got)
	}
}

func TestSelectExtractFilesExpandsDirectory(t *testing.T) {
	available := []domain.ExtractFile{
		{Path: "newmod/x.go", Status: domain.ExtractStatusUntracked},
		{Path: "newmod/sub/y.go", Status: domain.ExtractStatusUntracked},
		{Path: "newmodule.go", Status: domain.ExtractStatusUntracked},
		{Path: "other.go", Status: domain.ExtractStatusModified},
	}

	for _, arg := range []string{"newmod/", "newmod"} {
		got, err := rules.SelectExtractFiles(rules.SelectExtractFilesParams{
			Available: available,
			Paths:     []string{arg},
		})
		if err != nil {
			t.Fatalf("SelectExtractFiles(%q): %v", arg, err)
		}
		if len(got) != 2 || got[0].Path != "newmod/x.go" || got[1].Path != "newmod/sub/y.go" {
			t.Fatalf("SelectExtractFiles(%q) = %+v, want the two files under newmod/", arg, got)
		}
	}
}

func TestSelectExtractFilesDedupesOverlappingPaths(t *testing.T) {
	available := []domain.ExtractFile{
		{Path: "newmod/x.go", Status: domain.ExtractStatusUntracked},
		{Path: "newmod/y.go", Status: domain.ExtractStatusUntracked},
	}

	got, err := rules.SelectExtractFiles(rules.SelectExtractFilesParams{
		Available: available,
		Paths:     []string{"newmod/", "newmod/x.go"},
	})
	if err != nil {
		t.Fatalf("SelectExtractFiles: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d files, want 2 (x.go must not be selected twice)", len(got))
	}
}

func TestSelectExtractFilesRejectsUnknownPath(t *testing.T) {
	available := []domain.ExtractFile{{Path: "a.txt", Status: domain.ExtractStatusModified}}

	_, err := rules.SelectExtractFiles(rules.SelectExtractFilesParams{
		Available: available,
		Paths:     []string{"ghost.txt"},
	})
	if !errors.Is(err, domain.ErrNoFilesSelected) {
		t.Fatalf("err = %v, want ErrNoFilesSelected", err)
	}
}
