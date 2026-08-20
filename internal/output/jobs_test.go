package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

// TestWriteRunConfigJSON_LowercaseKeys ensures the wire format uses lowercase
// field names matching run.schema.json ("job", "profile", "name", "kind"…).
// This is a breaking change from the pre-json-tag behaviour where Go's default
// encoder used exported field names ("Jobs", "Name", etc.).
func TestWriteRunConfigJSON_LowercaseKeys(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm dev", Cwd: "."},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "full", Jobs: []string{"dev"}, Default: true},
		},
	}

	var buf bytes.Buffer
	if err := WriteRunConfigJSON(&buf, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw := buf.String()

	// Verify lowercase JSON keys.
	for _, key := range []string{`"job"`, `"profile"`, `"name"`, `"kind"`, `"cmd"`, `"cwd"`, `"jobs"`, `"default"`} {
		if !strings.Contains(raw, key) {
			t.Errorf("expected key %s in JSON output\noutput: %s", key, raw)
		}
	}

	// Verify PascalCase keys are absent.
	for _, key := range []string{`"Jobs"`, `"Profiles"`, `"Name"`, `"Kind"`, `"Cmd"`, `"Cwd"`} {
		if strings.Contains(raw, key) {
			t.Errorf("unexpected PascalCase key %s in JSON output\noutput: %s", key, raw)
		}
	}
}

// TestWriteRunConfigJSON_Roundtrip verifies that export → unmarshal is lossless.
func TestWriteRunConfigJSON_Roundtrip(t *testing.T) {
	orig := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm dev", Cwd: "."},
			{Name: "build", Kind: domain.JobKindTask, Cmd: "pnpm build"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "ci", Jobs: []string{"build"}, Default: false},
		},
	}

	var buf bytes.Buffer
	if err := WriteRunConfigJSON(&buf, orig); err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var got domain.RunConfig
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(got.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(got.Jobs))
	}
	if got.Jobs[0].Name != "dev" || got.Jobs[0].Kind != domain.JobKindService {
		t.Errorf("job[0] mismatch: %+v", got.Jobs[0])
	}
	if len(got.Profiles) != 1 || got.Profiles[0].Name != "ci" {
		t.Errorf("profile mismatch: %+v", got.Profiles)
	}
}

func TestWriteImportResultText(t *testing.T) {
	var buf bytes.Buffer
	result := ImportResult{
		Added:   []string{"dev", "build"},
		Skipped: []string{"old"},
	}

	WriteImportResultText(&buf, result)

	out := buf.String()
	for _, want := range []string{"Imported dev", "Imported build", "old already present"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output: %q", want, out)
		}
	}
}

func TestWriteImportResultJSON(t *testing.T) {
	var buf bytes.Buffer
	result := ImportResult{Added: []string{"a"}, Skipped: []string{"b"}}

	if err := WriteImportResultJSON(&buf, result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got ImportResult
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Added) != 1 || got.Added[0] != "a" {
		t.Errorf("expected Added=[a], got %v", got.Added)
	}
}

func TestWriteImportResultText_NothingToImport(t *testing.T) {
	var buf bytes.Buffer
	WriteImportResultText(&buf, ImportResult{})
	if !strings.Contains(buf.String(), "Nothing to import") {
		t.Errorf("expected nothing-to-import message: %q", buf.String())
	}
}

func TestFormatRunningJobs_ShowsUptime(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	out := FormatRunningJobs(FormatRunningJobsParams{
		Jobs: []domain.JobInfo{{
			Name:      "dev",
			Kind:      domain.JobKindService,
			Status:    domain.JobStatusRunning,
			PID:       4242,
			WorkDir:   "/work/feat",
			StartedAt: now.Add(-5 * time.Minute),
		}},
		Now: now,
	})

	if !strings.Contains(out, "UPTIME") {
		t.Errorf("expected an UPTIME column, got:\n%s", out)
	}
	if !strings.Contains(out, "5m") {
		t.Errorf("expected the uptime 5m, got:\n%s", out)
	}
	if row := strings.Fields(strings.Split(out, "\n")[1]); len(row) != 6 {
		t.Errorf("expected 6 filled cells, got %q", row)
	}
}

// TestFormatRunningJobs_NoUptimeWithoutARunningStart pins the two cases where
// the column stays blank rather than counting: a job the daemon never spawned,
// and one whose run is over.
func TestFormatRunningJobs_NoUptimeWithoutARunningStart(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name      string
		status    domain.JobStatus
		startedAt time.Time
	}{
		{"jamais démarré", domain.JobStatusRunning, time.Time{}},
		{"arrêté", domain.JobStatusStopped, now.Add(-5 * time.Minute)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			job := domain.JobInfo{
				Name:      "dev",
				Kind:      domain.JobKindService,
				Status:    c.status,
				PID:       7,
				WorkDir:   "/work/feat",
				StartedAt: c.startedAt,
			}
			out := FormatRunningJobs(FormatRunningJobsParams{Jobs: []domain.JobInfo{job}, Now: now})
			row := strings.Fields(strings.Split(out, "\n")[1])
			if len(row) != 5 {
				t.Errorf("expected 5 filled cells (no uptime), got %q", row)
			}
		})
	}
}

func TestWriteRunningJobsJSON_ExposesStartAndExit(t *testing.T) {
	startedAt := time.Date(2026, 8, 20, 11, 0, 0, 0, time.UTC)
	code := 3

	var buf bytes.Buffer
	err := WriteRunningJobsJSON(&buf, []domain.JobInfo{
		{Name: "dev", Status: domain.JobStatusCrashed, StartedAt: startedAt, ExitCode: &code},
		{Name: "ghost", Status: domain.JobStatusStopped},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got []domain.JobInfo
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !got[0].StartedAt.Equal(startedAt) {
		t.Errorf("started_at = %v, want %v", got[0].StartedAt, startedAt)
	}
	if got[0].ExitCode == nil || *got[0].ExitCode != code {
		t.Errorf("exit_code = %v, want %d", got[0].ExitCode, code)
	}

	// A job the daemon never ran carries neither key rather than an epoch date
	// and a code that would read as a clean exit.
	raw := buf.String()
	for _, key := range []string{`"started_at":"0001`, `"exit_code":0`} {
		if strings.Contains(raw, key) {
			t.Errorf("unexpected %s in %s", key, raw)
		}
	}
	if got[1].ExitCode != nil {
		t.Errorf("exit_code = %v, want nil", *got[1].ExitCode)
	}
}
