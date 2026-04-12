package output

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestWriteServiceResultsJSON(t *testing.T) {
	results := []ServiceActionResult{
		{Name: "api", Status: domain.ServiceActionStarted},
		{Name: "web", Status: domain.ServiceActionError, Message: "boom"},
	}
	var buf bytes.Buffer
	if err := WriteServiceResultsJSON(&buf, results); err != nil {
		t.Fatal(err)
	}
	var got []ServiceActionResult
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 2 || got[0].Name != "api" || got[1].Message != "boom" {
		t.Errorf("roundtrip mismatch: %+v", got)
	}
}

func TestWriteServiceResultsJSON_NilIsEmptyArray(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteServiceResultsJSON(&buf, nil); err != nil {
		t.Fatal(err)
	}
	if bytes.TrimSpace(buf.Bytes())[0] != '[' {
		t.Errorf("expected [...], got: %s", buf.String())
	}
}

func TestWritePRCheckoutJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := WritePRCheckoutJSON(&buf, PRCheckoutJSON{Number: 7, Branch: "feat/x", Path: "/tmp/x"}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["number"].(float64) != 7 || got["branch"] != "feat/x" || got["path"] != "/tmp/x" {
		t.Errorf("wrong payload: %v", got)
	}
}
