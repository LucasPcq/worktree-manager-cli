package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

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
