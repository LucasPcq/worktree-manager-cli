//go:build darwin

package proxy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestPlanRendersTheAgentWithBothLoopbacks(t *testing.T) {
	plan, err := NewRedirector(RedirectorParams{Root: t.TempDir()}).Plan(PlanParams{BindPort: 4000})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Files) != 1 {
		t.Fatalf("un seul fichier, chez l'utilisateur : %+v", plan.Files)
	}

	plist := plan.Files[0].Content
	for _, want := range []string{
		domain.ProxyPlistLabel,
		domain.ProxySocketKey,
		"<string>127.0.0.1</string>",
		"<string>::1</string>",
		"<string>80</string>",
		"--target</string><string>4000</string>",
	} {
		if !strings.Contains(plist, want) {
			t.Errorf("le plist doit contenir %q:\n%s", want, plist)
		}
	}
	if strings.Contains(plist, "0.0.0.0") {
		t.Error("la socket reste sur la loopback : 0.0.0.0 publierait chaque worktree sur le réseau")
	}
}

func TestPlanWritesInsideTheHomeDirectory(t *testing.T) {
	home := t.TempDir()

	plan, err := NewRedirector(RedirectorParams{Root: home}).Plan(PlanParams{BindPort: 4000})
	if err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(home, domain.ProxyAgentDir, domain.ProxyPlistName)
	if plan.Files[0].Path != want {
		t.Errorf("got %q, want %q", plan.Files[0].Path, want)
	}
	if !strings.Contains(plan.Script, domain.LaunchctlBin) {
		t.Errorf("le récap annonce la commande launchctl: %q", plan.Script)
	}
}

func TestInspectReadsTheDeclaredStateWithoutLaunchctl(t *testing.T) {
	home := t.TempDir()
	r := NewRedirector(RedirectorParams{Root: home})

	if r.Inspect().Installed {
		t.Error("home vierge : rien n'est installé")
	}

	plan, err := r.Plan(PlanParams{BindPort: 4321})
	if err != nil {
		t.Fatal(err)
	}
	if mkErr := os.MkdirAll(filepath.Dir(plan.Files[0].Path), 0o755); mkErr != nil {
		t.Fatal(mkErr)
	}
	if writeErr := os.WriteFile(plan.Files[0].Path, []byte(plan.Files[0].Content), 0o644); writeErr != nil {
		t.Fatal(writeErr)
	}

	status := r.Inspect()
	if !status.Installed {
		t.Error("le plist posé : installé")
	}
	if status.BindPort != 4321 {
		t.Errorf("le port cible se relit dans le plist : got %d", status.BindPort)
	}
	if !status.Supported {
		t.Error("darwin est supporté")
	}
}
