package output

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestWritePRListJSON_Shape(t *testing.T) {
	prs := []domain.PRInfo{
		{
			Number:    7,
			Title:     "feat: x",
			Author:    "alice",
			Branch:    "feat/x",
			State:     "OPEN",
			Draft:     false,
			CreatedAt: time.Now().UTC(),
			URL:       "http://x/7",
			CIStatus:  domain.CIStatusPassing,
			Reviews:   []domain.ReviewInfo{{User: "bob", State: "APPROVED"}},
		},
	}

	var buf bytes.Buffer
	if err := WritePRListJSON(&buf, prs); err != nil {
		t.Fatal(err)
	}
	var got []domain.PRInfo
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 1 || got[0].Number != 7 || got[0].CIStatus != domain.CIStatusPassing {
		t.Errorf("wrong payload: %+v", got)
	}
	if len(got[0].Reviews) != 1 || got[0].Reviews[0].User != "bob" {
		t.Errorf("review not roundtripped: %+v", got[0].Reviews)
	}
}

func TestWritePRListJSON_EmptyIsArray(t *testing.T) {
	var buf bytes.Buffer
	if err := WritePRListJSON(&buf, nil); err != nil {
		t.Fatal(err)
	}
	if bytes.TrimSpace(buf.Bytes())[0] != '[' {
		t.Errorf("expected array, got: %s", buf.String())
	}
}
