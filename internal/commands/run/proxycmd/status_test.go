package proxycmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()

	var buf bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestStatusJSONCarriesBothPorts(t *testing.T) {
	out, err := runCmd(t, domain.CmdStatus, "--"+domain.FlagOutput, domain.OutputJSON)
	if err != nil {
		t.Fatal(err)
	}

	var status domain.ProxyStatus
	if unmarshalErr := json.Unmarshal([]byte(out), &status); unmarshalErr != nil {
		t.Fatalf("sortie non-JSON: %v\n%s", unmarshalErr, out)
	}
	if status.ConfiguredPort != domain.ProxyDefaultPort {
		t.Errorf("le port configuré doit toujours être présent : got %d", status.ConfiguredPort)
	}
	if status.PublicPort == 0 {
		t.Error("le port public doit être résolu")
	}
}

func TestInstallRefusesJSON(t *testing.T) {
	_, err := runCmd(t, domain.CmdInstall, "--"+domain.FlagOutput, domain.OutputJSON, "--"+domain.FlagYes)

	if !errors.Is(err, domain.ErrProxyInstallNeedsTTY) {
		t.Errorf("got %v, want ErrProxyInstallNeedsTTY", err)
	}
}

func TestUninstallRefusesWithoutATerminal(t *testing.T) {
	_, err := runCmd(t, domain.CmdUninstall, "--"+domain.FlagYes)

	if !errors.Is(err, domain.ErrProxyInstallNeedsTTY) {
		t.Errorf("got %v, want ErrProxyInstallNeedsTTY", err)
	}
}
