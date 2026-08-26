package output_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/output"
)

func decodeResults(t *testing.T, outcome runlogs.Outcome) []domain.JobActionResult {
	t.Helper()
	var buf bytes.Buffer
	if err := output.WriteRunOutcomeJSON(&buf, outcome); err != nil {
		t.Fatalf("write: %v", err)
	}
	var results []domain.JobActionResult
	if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
		t.Fatalf("decode %s: %v", buf.String(), err)
	}
	return results
}

func TestRunOutcomeJSONCarriesThePortVerdicts(t *testing.T) {
	// The probe costs the machine surface its whole budget; dropping the verdict
	// leaves an agent reading "started" with no way to learn the port is silent.
	results := decodeResults(t, runlogs.Outcome{
		Results: []domain.JobActionResult{{Name: "web", Status: domain.JobActionStarted}},
		Probes: []domain.PortProbe{
			{Job: "web", Name: "WEB_PORT", Port: 5183, Status: domain.PortSilent, BaseListening: 5173},
		},
	})

	if len(results) != 1 || len(results[0].Ports) != 1 {
		t.Fatalf("expected the probe on its job, got %+v", results)
	}
	probe := results[0].Ports[0]
	if probe.Status != domain.PortSilent || probe.Port != 5183 || probe.BaseListening != 5173 {
		t.Errorf("probe = %+v, want the silent verdict with its base hint", probe)
	}
}

func TestRunOutcomeJSONPutsEachProbeOnItsOwnJob(t *testing.T) {
	results := decodeResults(t, runlogs.Outcome{
		Results: []domain.JobActionResult{
			{Name: "api", Status: domain.JobActionStarted},
			{Name: "web", Status: domain.JobActionStarted},
		},
		Probes: []domain.PortProbe{
			{Job: "web", Name: "WEB_PORT", Port: 5183, Status: domain.PortListening},
		},
	})

	if len(results[0].Ports) != 0 {
		t.Errorf("api was not probed, yet carries %+v", results[0].Ports)
	}
	if len(results[1].Ports) != 1 {
		t.Errorf("web's probe did not reach it: %+v", results[1])
	}
}

func TestRunOutcomeJSONOmitsPortsWhenNothingWasProbed(t *testing.T) {
	var buf bytes.Buffer
	if err := output.WriteRunOutcomeJSON(&buf, runlogs.Outcome{
		Results: []domain.JobActionResult{{Name: "seed", Status: domain.JobActionDone}},
	}); err != nil {
		t.Fatalf("write: %v", err)
	}

	if bytes.Contains(buf.Bytes(), []byte("ports")) {
		t.Errorf("a run with no probe must not emit an empty field:\n%s", buf.String())
	}
}
