package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestComposePortVarName(t *testing.T) {
	tests := []struct {
		name    string
		service string
		taken   []string
		want    string
	}{
		{"plain service", "postgres", nil, "POSTGRES_PORT"},
		{"dashes become underscores", "my-redis", nil, "MY_REDIS_PORT"},
		{"dots become underscores", "api.gateway", nil, "API_GATEWAY_PORT"},
		{"a leading digit is prefixed", "2fa", nil, "S2FA_PORT"},
		{"second port of a service takes the container port", "postgres", []string{"POSTGRES_PORT"}, "POSTGRES_PORT_9187"},
		{"a third collision falls back to a counter", "postgres", []string{"POSTGRES_PORT", "POSTGRES_PORT_9187"}, "POSTGRES_PORT_9187_2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taken := map[string]bool{}
			for _, k := range tt.taken {
				taken[k] = true
			}
			got := ComposePortVarName(ComposePortVarNameParams{
				Service:   tt.service,
				Container: 9187,
				Taken:     taken,
			})
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
			if !IsEnvVarName(got) {
				t.Errorf("%q is not a usable environment variable name", got)
			}
		})
	}
}

func TestComposeShortPortFrozen(t *testing.T) {
	tests := []struct {
		name            string
		mapping         string
		wantBase        int
		wantContainer   int
		wantReplacement string
	}{
		{"plain", "5432:5432", 5432, 5432, `"${POSTGRES_PORT:-5432}:5432"`},
		{"distinct sides", "15432:5432", 15432, 5432, `"${POSTGRES_PORT:-15432}:5432"`},
		{"host ip is kept", "127.0.0.1:8025:8025", 8025, 8025, `"127.0.0.1:${POSTGRES_PORT:-8025}:8025"`},
		{"protocol is kept", "5432:5432/udp", 5432, 5432, `"${POSTGRES_PORT:-5432}:5432/udp"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComposeShortPort(ComposeShortPortParams{Service: "postgres", Mapping: tt.mapping})
			if got.Status != domain.ComposePortFrozen {
				t.Fatalf("status = %q, want frozen (%s)", got.Status, got.Reason)
			}
			if got.Base != tt.wantBase || got.Container != tt.wantContainer {
				t.Errorf("base/container = %d/%d, want %d/%d", got.Base, got.Container, tt.wantBase, tt.wantContainer)
			}
			if got.Var != "POSTGRES_PORT" {
				t.Errorf("var = %q, want POSTGRES_PORT", got.Var)
			}
			if got.Replacement != tt.wantReplacement {
				t.Errorf("replacement = %s, want %s", got.Replacement, tt.wantReplacement)
			}
		})
	}
}

func TestComposeShortPortTemplated(t *testing.T) {
	tests := []struct {
		name     string
		mapping  string
		wantVar  string
		wantBase int
	}{
		{"braced with a default", "${DB_PORT:-5432}:5432", "DB_PORT", 5432},
		{"braced with a hard default", "${DB_PORT-5432}:5432", "DB_PORT", 5432},
		{"no default falls back to the container port", "${DB_PORT}:5432", "DB_PORT", 5432},
		{"bare variable", "$DB_PORT:5432", "DB_PORT", 5432},
		{"host ip and a variable", "127.0.0.1:${DB_PORT:-15432}:5432", "DB_PORT", 15432},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComposeShortPort(ComposeShortPortParams{Service: "db", Mapping: tt.mapping})
			if got.Status != domain.ComposePortTemplated {
				t.Fatalf("status = %q, want templated (%s)", got.Status, got.Reason)
			}
			if got.Var != tt.wantVar || got.Base != tt.wantBase {
				t.Errorf("got %s=%d, want %s=%d", got.Var, got.Base, tt.wantVar, tt.wantBase)
			}
			if got.Replacement != "" {
				t.Errorf("a templated mapping is never rewritten, got %q", got.Replacement)
			}
		})
	}
}

func TestComposeShortPortUnsupported(t *testing.T) {
	tests := []struct {
		name    string
		mapping string
		reason  string
	}{
		{"container port only", "5432", "no host port"},
		{"host range", "3000-3005:3000-3005", "port range"},
		{"container range", "3000:3000-3005", "port range"},
		{"host port is not a number", "abc:5432", "not a port number"},
		{"out of range", "70000:5432", "outside"},
		{"variable is not an env name", "${1BAD}:5432", "not a valid environment variable name"},
		{"empty", "", "no host port"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComposeShortPort(ComposeShortPortParams{Service: "svc", Mapping: tt.mapping})
			if got.Status != domain.ComposePortUnsupported {
				t.Fatalf("status = %q, want unsupported", got.Status)
			}
			if !strings.Contains(got.Reason, tt.reason) {
				t.Errorf("reason = %q, want it to mention %q", got.Reason, tt.reason)
			}
		})
	}
}

func TestComposeLongPort(t *testing.T) {
	frozen := ComposeLongPort(ComposeLongPortParams{Service: "web", Published: "8080", Target: "80"})
	if frozen.Status != domain.ComposePortFrozen {
		t.Fatalf("status = %q, want frozen (%s)", frozen.Status, frozen.Reason)
	}
	if frozen.Var != "WEB_PORT" || frozen.Base != 8080 || frozen.Container != 80 {
		t.Errorf("got %s=%d target %d, want WEB_PORT=8080 target 80", frozen.Var, frozen.Base, frozen.Container)
	}
	if frozen.Replacement != `"${WEB_PORT:-8080}"` {
		t.Errorf("replacement = %s, want \"${WEB_PORT:-8080}\"", frozen.Replacement)
	}

	templated := ComposeLongPort(ComposeLongPortParams{Service: "web", Published: "${WEB_PORT:-8080}", Target: "80"})
	if templated.Status != domain.ComposePortTemplated || templated.Var != "WEB_PORT" || templated.Base != 8080 {
		t.Errorf("got %q %s=%d, want templated WEB_PORT=8080", templated.Status, templated.Var, templated.Base)
	}

	missing := ComposeLongPort(ComposeLongPortParams{Service: "web", Target: "80"})
	if missing.Status != domain.ComposePortUnsupported || !strings.Contains(missing.Reason, "no host port") {
		t.Errorf("a long mapping without published is unsupported, got %q %q", missing.Status, missing.Reason)
	}
}

const composeSource = `services:
  postgres:          # la base
    image: postgres:16
    ports:
      - "5432:5432"
      - 9187:9187
  web:
    ports:
      - target: 80
        published: 8080
`

func TestApplyComposePortPatches(t *testing.T) {
	patched, err := ApplyComposePortPatches(ApplyComposePortPatchesParams{
		Content: composeSource,
		Bindings: []domain.ComposePortBinding{
			{Line: 5, Column: 9, Token: `"5432:5432"`, Replacement: `"${POSTGRES_PORT:-5432}:5432"`},
			{Line: 6, Column: 9, Token: "9187:9187", Replacement: `"${POSTGRES_PORT_9187:-9187}:9187"`},
			{Line: 10, Column: 20, Token: "8080", Replacement: `"${WEB_PORT:-8080}"`},
		},
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	want := `services:
  postgres:          # la base
    image: postgres:16
    ports:
      - "${POSTGRES_PORT:-5432}:5432"
      - "${POSTGRES_PORT_9187:-9187}:9187"
  web:
    ports:
      - target: 80
        published: "${WEB_PORT:-8080}"
`
	if patched != want {
		t.Errorf("patched file diverges:\ngot:\n%s\nwant:\n%s", patched, want)
	}
}

func TestApplyComposePortPatchesRefusesAMovedToken(t *testing.T) {
	_, err := ApplyComposePortPatches(ApplyComposePortPatchesParams{
		Content:  composeSource,
		Bindings: []domain.ComposePortBinding{{File: "docker-compose.yml", Line: 5, Column: 9, Token: `"6379:6379"`, Replacement: `x`}},
	})
	if err == nil {
		t.Fatal("a token that is no longer where it was scanned must abort the patch")
	}
	if !strings.Contains(err.Error(), "docker-compose.yml") || !strings.Contains(err.Error(), "5") {
		t.Errorf("error must name the file and the line, got %v", err)
	}
}

func TestApplyComposePortPatchesRefusesAnOutOfRangeLine(t *testing.T) {
	_, err := ApplyComposePortPatches(ApplyComposePortPatchesParams{
		Content:  composeSource,
		Bindings: []domain.ComposePortBinding{{Line: 99, Column: 1, Token: "x", Replacement: "y"}},
	})
	if err == nil {
		t.Fatal("a line past the end of the file must abort the patch")
	}
}

func TestApplyComposePortPatchesLeavesTheFileAloneWithoutBindings(t *testing.T) {
	got, err := ApplyComposePortPatches(ApplyComposePortPatchesParams{Content: composeSource})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got != composeSource {
		t.Error("no binding must mean no rewrite")
	}
}
