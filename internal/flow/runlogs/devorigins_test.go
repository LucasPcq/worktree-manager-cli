package runlogs_test

import (
	"context"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

func runNext(t *testing.T, params runlogs.RunParams) *runlogstest.Sink {
	t.Helper()
	sink := &runlogstest.Sink{}
	params.Sink = sink
	params.Service = &runlogstest.Service{Ports: map[string]map[string]int{"web": {"PORT": 3010}}}
	params.Jobs = []domain.JobConfig{{
		Name:  "web",
		Kind:  domain.JobKindService,
		Cmd:   "pnpm run dev --port ${PORT}",
		Ports: map[string]int{"PORT": 3000},
		URL:   &domain.JobURLConfig{Port: "PORT"},
	}}
	params.Env = map[string]string{domain.EnvWorktree: "feat"}
	if _, err := runlogs.Run(context.Background(), params); err != nil {
		t.Fatalf("Run: %v", err)
	}
	return sink
}

func fixesFor(sink *runlogstest.Sink) []domain.DevOriginFix {
	for _, e := range sink.Events {
		if e.Phase == runlogs.PhaseStarted {
			return e.DevOrigins
		}
	}
	return nil
}

func TestRunReportsAMissingAllowedDevOrigins(t *testing.T) {
	sink := runNext(t, runlogs.RunParams{
		Project:   "myapp",
		ProxyPort: 4000,
		NextConfig: func(domain.JobConfig) (string, string) {
			return "apps/web/next.config.ts", "export default { reactStrictMode: true }\n"
		},
	})

	fixes := fixesFor(sink)
	if len(fixes) != 1 {
		t.Fatalf("fixes = %v, want the one line the project is missing", fixes)
	}
	if !strings.Contains(fixes[0].Line, domain.DevOriginsKey) || !strings.Contains(fixes[0].Line, "apps/web/next.config.ts") {
		t.Errorf("line = %q, want the option and the file to edit named", fixes[0].Line)
	}
}

func TestRunSaysNothingWhenTheProxyIsOff(t *testing.T) {
	// Under its own port there is no other host to allow, so the warning would
	// be noise about a problem the run does not have.
	sink := runNext(t, runlogs.RunParams{
		Project: "myapp",
		NextConfig: func(domain.JobConfig) (string, string) {
			return "apps/web/next.config.ts", "export default {}\n"
		},
	})

	if fixes := fixesFor(sink); len(fixes) != 0 {
		t.Errorf("fixes = %v, want none", fixes)
	}
}

func TestRunSaysNothingWhenAllowedDevOriginsIsAlreadyThere(t *testing.T) {
	sink := runNext(t, runlogs.RunParams{
		Project:   "myapp",
		ProxyPort: 4000,
		NextConfig: func(domain.JobConfig) (string, string) {
			return "apps/web/next.config.ts", `export default { allowedDevOrigins: ["*.localhost:4000"] }` + "\n"
		},
	})

	if fixes := fixesFor(sink); len(fixes) != 0 {
		t.Errorf("fixes = %v, want none", fixes)
	}
}
