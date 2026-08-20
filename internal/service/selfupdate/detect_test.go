package selfupdate_test

import (
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/selfupdate"
)

func TestDetectInstallReportsSourceForDevBuild(t *testing.T) {
	got := selfupdate.DetectInstall(domain.Version)
	if got.Method != domain.InstallSource {
		t.Fatalf("Method = %q, want %q", got.Method, domain.InstallSource)
	}
}

func TestDetectInstallResolvesARealBinaryPath(t *testing.T) {
	got := selfupdate.DetectInstall("0.26.1")
	if got.BinaryPath == "" {
		t.Fatal("BinaryPath is empty; DetectInstall must resolve the running executable")
	}
	if !filepath.IsAbs(got.BinaryPath) {
		t.Fatalf("BinaryPath = %q, want an absolute path", got.BinaryPath)
	}
}
