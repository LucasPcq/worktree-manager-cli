package rules

import (
	"reflect"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestUseRunView(t *testing.T) {
	tests := []struct {
		name   string
		params RunSurfaceParams
		want   bool
	}{
		{
			name:   "a human reading a terminal gets the view",
			params: RunSurfaceParams{Format: domain.OutputText, TTY: true},
			want:   true,
		},
		{
			name:   "an empty format is the text default",
			params: RunSurfaceParams{TTY: true},
			want:   true,
		},
		{
			name:   "json never takes the terminal over",
			params: RunSurfaceParams{Format: domain.OutputJSON, TTY: true},
			want:   false,
		},
		{
			name:   "a pipe never takes the terminal over",
			params: RunSurfaceParams{Format: domain.OutputText},
			want:   false,
		},
		{
			name:   "-d gives the terminal back",
			params: RunSurfaceParams{Format: domain.OutputText, TTY: true, Detach: true},
			want:   false,
		},
		{
			name:   "an inline job keeps the scrollback",
			params: RunSurfaceParams{Format: domain.OutputText, TTY: true, Inline: true},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UseRunView(tt.params); got != tt.want {
				t.Errorf("UseRunView(%+v) = %v, want %v", tt.params, got, tt.want)
			}
		})
	}
}

func TestRunsInline(t *testing.T) {
	if !RunsInline(domain.JobConfig{Name: "migrate", Kind: domain.JobKindTask}) {
		t.Error("a task runs inline")
	}
	if RunsInline(domain.JobConfig{Name: "api", Kind: domain.JobKindService}) {
		t.Error("a service does not hold the terminal")
	}
}

func TestStreamsStartOutput(t *testing.T) {
	tests := []struct {
		name string
		job  domain.JobConfig
		want bool
	}{
		{"task", domain.JobConfig{Name: "migrate", Kind: domain.JobKindTask}, true},
		{"detached launcher", domain.JobConfig{Name: "docker", Kind: domain.JobKindService, Stop: "docker compose down"}, true},
		{"backgrounded service", domain.JobConfig{Name: "api", Kind: domain.JobKindService}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StreamsStartOutput(tt.job); got != tt.want {
				t.Errorf("StreamsStartOutput(%+v) = %v, want %v", tt.job, got, tt.want)
			}
		})
	}
}

func TestWithFailureOutputFoldsTheOutputIntoTheFailedResult(t *testing.T) {
	results := []domain.JobActionResult{
		{Name: "docker", Status: domain.JobActionStarted},
		{Name: "migrate", Status: domain.JobActionError, Message: "task migrate failed: exit status 1"},
	}

	got := WithFailureOutput(FailureOutputParams{
		Results: results,
		Job:     "migrate",
		Output:  []byte("\n  relation \"users\" does not exist\n\n"),
	})

	want := []domain.JobActionResult{
		{Name: "docker", Status: domain.JobActionStarted},
		{Name: "migrate", Status: domain.JobActionError, Message: "task migrate failed: exit status 1\nrelation \"users\" does not exist"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("results %+v, want %+v", got, want)
	}
	if results[1].Message != "task migrate failed: exit status 1" {
		t.Fatalf("the input was mutated: %q", results[1].Message)
	}
}

func TestWithFailureOutputLeavesEverythingElseAlone(t *testing.T) {
	results := []domain.JobActionResult{
		{Name: "migrate", Status: domain.JobActionDone},
		{Name: "api", Status: domain.JobActionError, Message: "start api: connection refused"},
	}

	tests := []struct {
		name   string
		params FailureOutputParams
	}{
		{"no job failed", FailureOutputParams{Results: results, Output: []byte("noise")}},
		{"the job printed nothing", FailureOutputParams{Results: results, Job: "api"}},
		{"the job printed whitespace", FailureOutputParams{Results: results, Job: "api", Output: []byte(" \n\t")}},
		{"a job of another name", FailureOutputParams{Results: results, Job: "web", Output: []byte("boom")}},
		{"a result that did not fail", FailureOutputParams{Results: results, Job: "migrate", Output: []byte("boom")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, got := range WithFailureOutput(tt.params) {
				if got.Name == "api" && got.Message != "start api: connection refused" {
					t.Errorf("api message = %q, want it untouched", got.Message)
				}
				if got.Name == "migrate" && got.Message != "" {
					t.Errorf("migrate message = %q, want it untouched", got.Message)
				}
			}
		})
	}
}
