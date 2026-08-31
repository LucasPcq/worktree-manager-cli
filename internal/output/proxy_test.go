package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

var planFiles = []domain.ProxyPlannedFile{
	{Path: "/etc/pf.anchors/wtm", Content: "rdr pass on lo0 …\n", Change: "new file — redirects :80 to :4000"},
	{Path: "/etc/pf.conf", Content: "# Default PF configuration file.\nrdr-anchor \"wtm\"\n", Change: "2 lines added, inside a marked block"},
}

func TestProxyPlanReportShowsChangesNotContents(t *testing.T) {
	var buf bytes.Buffer

	ProxyPlanReport(&buf, ProxyPlanReportParams{Files: planFiles, Script: "set -e\npfctl -E", Reversible: true})

	out := buf.String()
	if strings.Contains(out, "# Default PF configuration file.") {
		t.Errorf("le récap par défaut ne déverse pas les contenus:\n%s", out)
	}
	for _, want := range []string{"/etc/pf.conf", "2 lines added", "pfctl -E", "uninstall", "--dry-run"} {
		if !strings.Contains(out, want) {
			t.Errorf("le récap doit contenir %q:\n%s", want, out)
		}
	}
}

func TestProxyPlanReportFullShowsContents(t *testing.T) {
	var buf bytes.Buffer

	ProxyPlanReport(&buf, ProxyPlanReportParams{Files: planFiles, Full: true, Reversible: true})

	out := buf.String()
	if !strings.Contains(out, "# Default PF configuration file.") {
		t.Errorf("--dry-run montre les contenus:\n%s", out)
	}
	if strings.Contains(out, "--dry-run") {
		t.Error("inutile de renvoyer vers --dry-run quand on y est déjà")
	}
}
