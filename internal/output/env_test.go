package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// A declined port pass must not re-print its table: the table was the proposal,
// and once the answer is no a result report listing what did not happen is noise.
func TestPrintEnvReportCollapsesADeclinedPortPass(t *testing.T) {
	plan := rules.PlanEnvPorts(rules.PlanEnvPortsParams{
		Links:  []domain.EnvPortLink{{File: ".env", Key: "DATABASE_URL", Job: "svc", Port: "POSTGRES_PORT"}},
		Bases:  map[domain.PortRef]int{{Job: "svc", Name: "POSTGRES_PORT"}: 5432},
		Offset: 10,
		Lines:  map[string][]domain.EnvLine{".env": rules.ParseEnv("DATABASE_URL=postgres://localhost:5432/app\n")},
	})
	result := domain.EnvSyncResult{
		Branch: "feat/x",
		Mode:   domain.EnvModeAdd,
		Files:  []domain.EnvFileResult{{Target: ".env", Applied: true}},
		Ports:  plan,
	}

	var declined bytes.Buffer
	PrintEnvReport(&declined, result)
	if strings.Contains(declined.String(), "BECOMES") {
		t.Errorf("a declined pass printed its table:\n%s", declined.String())
	}
	if !strings.Contains(declined.String(), "left alone") {
		t.Errorf("a declined pass never says so:\n%s", declined.String())
	}

	result.Ports.Applied = true
	var applied bytes.Buffer
	PrintEnvReport(&applied, result)
	if !strings.Contains(applied.String(), "BECOMES") {
		t.Errorf("an applied pass hid its table:\n%s", applied.String())
	}
}

// The report has to name what it ran on: a reader who omitted the argument and
// picked interactively otherwise gets no confirmation of what was reconciled.
func TestPrintEnvReportNamesTheWorktreeAndMode(t *testing.T) {
	var buf bytes.Buffer
	PrintEnvReport(&buf, domain.EnvSyncResult{
		Branch: "feat/x",
		Mode:   domain.EnvModeRefresh,
		Check:  true,
		Files:  []domain.EnvFileResult{{Target: ".env"}},
	})

	for _, want := range []string{"feat/x", "refresh", "read-only check"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("report never mentions %q:\n%s", want, buf.String())
		}
	}
}
