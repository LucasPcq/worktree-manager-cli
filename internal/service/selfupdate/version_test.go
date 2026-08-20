package selfupdate_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/selfupdate"
)

func TestResolveVersionKeepsTheLinkedVersion(t *testing.T) {
	if got := selfupdate.ResolveVersion("0.26.1"); got != "0.26.1" {
		t.Fatalf("ResolveVersion(0.26.1) = %q, want 0.26.1 (the ldflag wins)", got)
	}
}

// The test binary is built from this source tree, so the build info reports
// "(devel)" and the fallback must decline it rather than hand back a fake
// version. The go-install case — a real module version in the build info — is
// covered end to end in the PR checklist, not here: it cannot be produced from
// inside `go test`.
func TestResolveVersionDeclinesADevelBuild(t *testing.T) {
	if got := selfupdate.ResolveVersion(domain.Version); got != domain.Version {
		t.Fatalf("ResolveVersion(dev) = %q, want %q for a source build", got, domain.Version)
	}
}
