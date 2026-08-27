package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestJobURL(t *testing.T) {
	web := domain.JobConfig{Name: "web", Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "PORT"}}

	tests := []struct {
		name      string
		job       domain.JobConfig
		ports     map[string]int
		host      string
		proxyPort int
		want      string
	}{
		{name: "port résolu du worktree", job: web, ports: map[string]int{"PORT": 3010}, want: "http://localhost:3010"},
		{name: "worktree principal", job: web, ports: map[string]int{"PORT": 3000}, want: "http://localhost:3000"},
		{name: "job sans url", job: domain.JobConfig{Name: "db", Ports: map[string]int{"PG_PORT": 5432}}, ports: map[string]int{"PG_PORT": 5442}, want: ""},
		{name: "port absent des résolus", job: web, ports: map[string]int{"OTHER": 9000}, want: ""},
		{name: "aucun port résolu", job: web, want: ""},
		{
			name:      "forme nommée quand le proxy sert",
			job:       web,
			ports:     map[string]int{"PORT": 3010},
			host:      "web.feat-auth.myapp.localhost",
			proxyPort: 4000,
			want:      "http://web.feat-auth.myapp.localhost:4000",
		},
		{
			name:  "proxy éteint : la forme directe est la réponse honnête",
			job:   web,
			ports: map[string]int{"PORT": 3010},
			host:  "web.feat-auth.myapp.localhost",
			want:  "http://localhost:3010",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := JobURL(JobURLParams{Job: tt.job, Ports: tt.ports, Host: tt.host, ProxyPort: tt.proxyPort})
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
