//go:build darwin

package proxy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// fakeRoot builds a temp directory standing in for /, with the stock pf.conf.
func fakeRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "etc"), 0o755); err != nil {
		t.Fatal(err)
	}
	stock := "scrub-anchor \"com.apple/*\"\nnat-anchor \"com.apple/*\"\nrdr-anchor \"com.apple/*\"\nanchor \"com.apple/*\"\nload anchor \"com.apple\" from \"/etc/pf.anchors/com.apple\"\n"
	if err := os.WriteFile(filepath.Join(root, "etc", "pf.conf"), []byte(stock), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestPlanRendersTheAnchorWithTheBindPort(t *testing.T) {
	plan, err := NewRedirector(RedirectorParams{Root: fakeRoot(t)}).Plan(PlanParams{BindPort: 4000})
	if err != nil {
		t.Fatal(err)
	}

	anchor := fileNamed(t, plan, domain.ProxyAnchorPath)
	if !strings.Contains(anchor, "port 80 -> 127.0.0.1 port 4000") {
		t.Errorf("l'ancre doit rediriger 80 vers le port de bind:\n%s", anchor)
	}
	if !strings.Contains(anchor, "on lo0") {
		t.Errorf("la redirection est sur la loopback:\n%s", anchor)
	}
}

func TestPlanRendersThePlistWithItsLabel(t *testing.T) {
	plan, err := NewRedirector(RedirectorParams{Root: fakeRoot(t)}).Plan(PlanParams{BindPort: 4000})
	if err != nil {
		t.Fatal(err)
	}

	plist := fileNamed(t, plan, domain.ProxyPlistPath)
	if !strings.Contains(plist, domain.ProxyPlistLabel) {
		t.Errorf("le plist doit porter son label:\n%s", plist)
	}
	if !strings.Contains(plist, "RunAtLoad") {
		t.Errorf("le plist doit recharger pf au boot:\n%s", plist)
	}
}

func TestPlanRendersThePfConfWithBothBlocks(t *testing.T) {
	plan, err := NewRedirector(RedirectorParams{Root: fakeRoot(t)}).Plan(PlanParams{BindPort: 4000})
	if err != nil {
		t.Fatal(err)
	}

	conf := fileNamed(t, plan, domain.ProxyPfConfPath)
	if !strings.Contains(conf, domain.ProxyPfRdrAnchorLine) || !strings.Contains(conf, domain.ProxyPfLoadLine) {
		t.Errorf("les deux lignes doivent être là:\n%s", conf)
	}
}

func TestPlanScriptInstallsThenReloads(t *testing.T) {
	plan, err := NewRedirector(RedirectorParams{Root: fakeRoot(t)}).Plan(PlanParams{BindPort: 4000})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"install -m", domain.ProxyAnchorPath, domain.ProxyPfConfPath, domain.ProxyPlistPath, "launchctl bootstrap system", "pfctl -E -f"} {
		if !strings.Contains(plan.Script, want) {
			t.Errorf("le script doit contenir %q:\n%s", want, plan.Script)
		}
	}
	if strings.Index(plan.Script, "install -m") > strings.Index(plan.Script, "pfctl -E -f") {
		t.Error("les fichiers sont posés avant le rechargement de pf")
	}
}

func TestInspectReadsTheDeclaredStateWithoutRoot(t *testing.T) {
	root := fakeRoot(t)
	r := NewRedirector(RedirectorParams{Root: root})

	if r.Inspect().Installed {
		t.Error("racine vierge : rien n'est installé")
	}

	plan, err := r.Plan(PlanParams{BindPort: 4000})
	if err != nil {
		t.Fatal(err)
	}
	writePlanInto(t, root, plan)

	status := r.Inspect()
	if !status.Installed {
		t.Error("les trois artefacts posés : installé")
	}
	if status.BindPort != 4000 {
		t.Errorf("le port de bind se relit dans l'ancre : got %d", status.BindPort)
	}
	if !status.Supported {
		t.Error("darwin est supporté")
	}
}

func fileNamed(t *testing.T, plan Plan, path string) string {
	t.Helper()
	for _, f := range plan.Files {
		if f.Path == path {
			return f.Content
		}
	}
	t.Fatalf("le plan ne contient pas %s", path)
	return ""
}

func writePlanInto(t *testing.T, root string, plan Plan) {
	t.Helper()
	for _, f := range plan.Files {
		dest := filepath.Join(root, f.Path)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dest, []byte(f.Content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
