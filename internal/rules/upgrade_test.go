package rules_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestIsNewerVersion(t *testing.T) {
	cases := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{"patch bump", "0.26.1", "0.26.2", true},
		{"minor bump", "0.26.1", "0.27.0", true},
		{"major bump", "0.26.1", "1.0.0", true},
		{"equal", "0.26.1", "0.26.1", false},
		{"older", "0.27.0", "0.26.1", false},
		{"leading v on latest", "0.26.1", "v0.27.0", true},
		{"leading v on both", "v0.26.1", "v0.26.1", false},
		{"numeric not lexical", "0.9.0", "0.10.0", true},
		{"prerelease below its release", "1.0.0-rc1", "1.0.0", true},
		{"release above its prerelease", "1.0.0", "1.0.0-rc1", false},
		{"prerelease ordering", "1.0.0-rc1", "1.0.0-rc2", true},
		{"dev never notifies", "dev", "0.27.0", false},
		{"garbage current", "banana", "0.27.0", false},
		{"garbage latest", "0.26.1", "banana", false},
		{"empty latest", "0.26.1", "", false},
		{"short version", "1.0", "1.0.1", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rules.IsNewerVersion(tc.current, tc.latest); got != tc.want {
				t.Fatalf("IsNewerVersion(%q, %q) = %v, want %v", tc.current, tc.latest, got, tc.want)
			}
		})
	}
}

func TestNormalizeVersion(t *testing.T) {
	cases := []struct{ in, want string }{
		{"v0.26.1", "0.26.1"},
		{"0.26.1", "0.26.1"},
		{"", ""},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := rules.NormalizeVersion(tc.in); got != tc.want {
				t.Fatalf("NormalizeVersion(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestClassifyInstall(t *testing.T) {
	cases := []struct {
		name   string
		params rules.ClassifyInstallParams
		want   domain.InstallMethod
	}{
		{
			name: "homebrew arm mac",
			params: rules.ClassifyInstallParams{
				ExecPath:     "/opt/homebrew/bin/wtm",
				ResolvedPath: "/opt/homebrew/Cellar/wtm/0.26.1/bin/wtm",
				GoBinDir:     "/Users/x/go/bin",
				Version:      "0.26.1",
			},
			want: domain.InstallHomebrew,
		},
		{
			name: "homebrew intel mac",
			params: rules.ClassifyInstallParams{
				ExecPath:     "/usr/local/bin/wtm",
				ResolvedPath: "/usr/local/Cellar/wtm/0.26.1/bin/wtm",
				GoBinDir:     "/Users/x/go/bin",
				Version:      "0.26.1",
			},
			want: domain.InstallHomebrew,
		},
		{
			name: "linuxbrew",
			params: rules.ClassifyInstallParams{
				ExecPath:     "/home/x/.linuxbrew/bin/wtm",
				ResolvedPath: "/home/linuxbrew/.linuxbrew/Cellar/wtm/0.26.1/bin/wtm",
				GoBinDir:     "/home/x/go/bin",
				Version:      "0.26.1",
			},
			want: domain.InstallHomebrew,
		},
		{
			name: "go install",
			params: rules.ClassifyInstallParams{
				ExecPath:     "/Users/x/go/bin/wtm",
				ResolvedPath: "/Users/x/go/bin/wtm",
				GoBinDir:     "/Users/x/go/bin",
				Version:      "0.26.1",
			},
			want: domain.InstallGoInstall,
		},
		{
			name: "standalone in usr local bin",
			params: rules.ClassifyInstallParams{
				ExecPath:     "/usr/local/bin/wtm",
				ResolvedPath: "/usr/local/bin/wtm",
				GoBinDir:     "/Users/x/go/bin",
				Version:      "0.26.1",
			},
			want: domain.InstallStandalone,
		},
		{
			name: "user directory literally named Cellar is not brew",
			params: rules.ClassifyInstallParams{
				ExecPath:     "/Users/x/Cellar/wtm",
				ResolvedPath: "/Users/x/Cellar/wtm",
				GoBinDir:     "/Users/x/go/bin",
				Version:      "0.26.1",
			},
			want: domain.InstallStandalone,
		},
		{
			name: "dev build in go bin is source, not go-install",
			params: rules.ClassifyInstallParams{
				ExecPath:     "/Users/x/go/bin/wtm",
				ResolvedPath: "/Users/x/go/bin/wtm",
				GoBinDir:     "/Users/x/go/bin",
				Version:      "dev",
			},
			want: domain.InstallSource,
		},
		{
			name: "empty go bin dir does not swallow everything",
			params: rules.ClassifyInstallParams{
				ExecPath:     "/usr/local/bin/wtm",
				ResolvedPath: "/usr/local/bin/wtm",
				GoBinDir:     "",
				Version:      "0.26.1",
			},
			want: domain.InstallStandalone,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rules.ClassifyInstall(tc.params); got != tc.want {
				t.Fatalf("ClassifyInstall(%+v) = %q, want %q", tc.params, got, tc.want)
			}
		})
	}
}

func TestReleaseAssetName(t *testing.T) {
	cases := []struct {
		name   string
		params rules.ReleaseAssetNameParams
		want   string
	}{
		{"darwin arm64", rules.ReleaseAssetNameParams{Version: "0.26.1", GOOS: "darwin", GOARCH: "arm64"}, "wtm_0.26.1_darwin_arm64.tar.gz"},
		{"linux amd64", rules.ReleaseAssetNameParams{Version: "0.26.1", GOOS: "linux", GOARCH: "amd64"}, "wtm_0.26.1_linux_amd64.tar.gz"},
		{"tag with leading v is normalized", rules.ReleaseAssetNameParams{Version: "v0.26.1", GOOS: "darwin", GOARCH: "amd64"}, "wtm_0.26.1_darwin_amd64.tar.gz"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rules.ReleaseAssetName(tc.params); got != tc.want {
				t.Fatalf("ReleaseAssetName(%+v) = %q, want %q", tc.params, got, tc.want)
			}
		})
	}
}

func TestUpgradeCommandFor(t *testing.T) {
	cases := []struct {
		method domain.InstallMethod
		want   string
	}{
		{domain.InstallHomebrew, "brew upgrade LucasPcq/tap/wtm"},
		{domain.InstallGoInstall, "go install github.com/LucasPcq/wtm@latest"},
		{domain.InstallStandalone, "wtm upgrade"},
		{domain.InstallSource, "git pull && make install"},
	}

	for _, tc := range cases {
		t.Run(string(tc.method), func(t *testing.T) {
			if got := rules.UpgradeCommandFor(tc.method); got != tc.want {
				t.Fatalf("UpgradeCommandFor(%q) = %q, want %q", tc.method, got, tc.want)
			}
		})
	}
}
