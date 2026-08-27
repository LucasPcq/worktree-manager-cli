package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestJobURL(t *testing.T) {
	web := domain.JobConfig{Name: "web", Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "PORT"}}

	tests := []struct {
		name  string
		job   domain.JobConfig
		ports map[string]int
		want  string
	}{
		{"port résolu du worktree", web, map[string]int{"PORT": 3010}, "http://localhost:3010"},
		{"worktree principal", web, map[string]int{"PORT": 3000}, "http://localhost:3000"},
		{"job sans url", domain.JobConfig{Name: "db", Ports: map[string]int{"PG_PORT": 5432}}, map[string]int{"PG_PORT": 5442}, ""},
		{"port absent des résolus", web, map[string]int{"OTHER": 9000}, ""},
		{"aucun port résolu", web, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := JobURL(JobURLParams{Job: tt.job, Ports: tt.ports}); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
