package output

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
)

func TestWriteWorktreeListJSON_Shape(t *testing.T) {
	createdAt := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)
	var buf bytes.Buffer
	err := WriteWorktreeListJSON(&buf, WriteWorktreeListJSONParams{
		Statuses: []domain.WorktreeStatus{{
			Branch:       "feat/x",
			Path:         "/tmp/wt/x",
			IsParent:     false,
			IsDirty:      true,
			CommitsAhead: 3,
			CreatedAt:    createdAt,
		}},
		PRInfos: []domain.PRInfo{{Number: 42, URL: "http://x", Branch: "feat/x", State: "OPEN"}},
		Services: []process.ServiceInfo{{
			Name:    "api",
			WorkDir: "/tmp/wt/x",
			Status:  domain.ServiceStatusRunning,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	var got []domain.WorktreeListEntry
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, buf.String())
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	e := got[0]
	if e.Branch != "feat/x" || e.Path != "/tmp/wt/x" || !e.IsDirty || e.CommitsAhead != 3 {
		t.Errorf("core fields wrong: %+v", e)
	}
	if e.PR == nil || e.PR.Number != 42 || e.PR.URL != "http://x" {
		t.Errorf("pr wrong: %+v", e.PR)
	}
	if len(e.Services) != 1 || e.Services[0] != "api" {
		t.Errorf("services wrong: %v", e.Services)
	}
}

func TestWriteWorktreeListJSON_EmptyServicesIsEmptyArray(t *testing.T) {
	var buf bytes.Buffer
	err := WriteWorktreeListJSON(&buf, WriteWorktreeListJSONParams{
		Statuses: []domain.WorktreeStatus{{Branch: "main", Path: "/main"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"services": []`)) {
		t.Errorf("expected empty services array, got: %s", buf.String())
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"pr": null`)) {
		t.Errorf("expected null pr, got: %s", buf.String())
	}
}

func TestWriteWorktreeCleanJSON_Shape(t *testing.T) {
	var buf bytes.Buffer
	err := WriteWorktreeCleanJSON(&buf, WriteWorktreeCleanJSONParams{
		Branch: "feat/x",
		Path:   "/tmp/wt/x",
	})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["branch"] != "feat/x" || got["path"] != "/tmp/wt/x" {
		t.Errorf("wrong payload: %v", got)
	}
}
