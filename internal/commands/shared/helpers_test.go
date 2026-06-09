package shared

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestShowGHBanner(t *testing.T) {
	tests := []struct {
		name     string
		conn     domain.GHConnection
		wantSub  string // substring expected in the banner
		wantNone bool   // true when no banner should be printed
	}{
		{name: "ok prints nothing", conn: domain.GHConnectionOK, wantNone: true},
		{name: "not installed", conn: domain.GHConnectionNotInstalled, wantSub: "GitHub CLI not found"},
		{name: "not authenticated", conn: domain.GHConnectionNotAuthenticated, wantSub: "GitHub not connected"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			ShowGHBanner(&buf, tc.conn)
			got := buf.String()

			if tc.wantNone {
				if strings.TrimSpace(got) != "" {
					t.Errorf("expected no banner for %v, got %q", tc.conn, got)
				}
				return
			}
			if !strings.Contains(got, tc.wantSub) {
				t.Errorf("expected banner to contain %q, got %q", tc.wantSub, got)
			}
		})
	}
}
