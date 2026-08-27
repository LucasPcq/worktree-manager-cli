package jobcmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
)

func flagged(t *testing.T, port, host string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.Flags().String(domain.FlagURLPort, port, "")
	cmd.Flags().String(domain.FlagURLHost, host, "")
	return cmd
}

func TestURLFromFlags(t *testing.T) {
	got, err := urlFromFlags(flagged(t, "PORT", "api.app-1"))
	if err != nil {
		t.Fatalf("urlFromFlags: %v", err)
	}
	if got == nil || got.Port != "PORT" || got.Host != "api.app-1" {
		t.Errorf("got %+v, want PORT under api.app-1", got)
	}
}

func TestURLFromFlagsPublishesNothingByDefault(t *testing.T) {
	got, err := urlFromFlags(flagged(t, "", ""))
	if err != nil || got != nil {
		t.Errorf("got %+v, %v — a job publishes nothing unless asked", got, err)
	}
}

// A host with nothing to publish is a typo, not a job that publishes under it.
func TestURLFromFlagsRefusesAHostWithoutAPort(t *testing.T) {
	_, err := urlFromFlags(flagged(t, "", "api.app-1"))
	if err == nil || !strings.Contains(err.Error(), domain.FlagURLPort) {
		t.Errorf("err = %v, want one naming --%s", err, domain.FlagURLPort)
	}
}
