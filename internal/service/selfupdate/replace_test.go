package selfupdate_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/selfupdate"
)

func buildArchive(t *testing.T, payload string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	if err := tw.WriteHeader(&tar.Header{Name: "wtm", Mode: 0o755, Size: int64(len(payload))}); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if _, err := tw.Write([]byte(payload)); err != nil {
		t.Fatalf("write body: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}

	return buf.Bytes()
}

type releaseServer struct {
	*httptest.Server
	assetName string
}

func newReleaseServer(t *testing.T, archive []byte, checksum string) releaseServer {
	t.Helper()

	assetName := "wtm_0.27.0_darwin_arm64.tar.gz"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/" + assetName:
			_, _ = w.Write(archive)
		case "/checksums.txt":
			fmt.Fprintf(w, "%s  %s\n", checksum, assetName)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	return releaseServer{Server: srv, assetName: assetName}
}

func (s releaseServer) release() domain.ReleaseInfo {
	return domain.ReleaseInfo{
		Version: "0.27.0",
		Tag:     "v0.27.0",
		Assets: []domain.ReleaseAsset{
			{Name: s.assetName, URL: s.URL + "/" + s.assetName},
			{Name: domain.ChecksumsFileName, URL: s.URL + "/checksums.txt"},
		},
	}
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func seedBinary(t *testing.T) (dir string, target string) {
	t.Helper()

	dir = t.TempDir()
	target = filepath.Join(dir, "wtm")
	if err := os.WriteFile(target, []byte("OLD BINARY"), 0o755); err != nil {
		t.Fatalf("seed binary: %v", err)
	}

	return dir, target
}

func TestReplaceBinarySwapsTheFile(t *testing.T) {
	archive := buildArchive(t, "NEW BINARY")
	srv := newReleaseServer(t, archive, sha256Hex(archive))
	defer srv.Close()

	dir, target := seedBinary(t)

	err := selfupdate.ReplaceBinary(selfupdate.ReplaceBinaryParams{
		Release:    srv.release(),
		BinaryPath: target,
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Timeout:    5 * time.Second,
	})
	if err != nil {
		t.Fatalf("ReplaceBinary: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "NEW BINARY" {
		t.Fatalf("binary content = %q, want %q", got, "NEW BINARY")
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("mode = %v, want 0755", info.Mode().Perm())
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("directory holds %d entries, want 1 — a temp file leaked", len(entries))
	}
}

func TestReplaceBinaryLeavesOriginalIntactOnChecksumMismatch(t *testing.T) {
	archive := buildArchive(t, "NEW BINARY")
	srv := newReleaseServer(t, archive, sha256Hex([]byte("something else")))
	defer srv.Close()

	dir, target := seedBinary(t)

	err := selfupdate.ReplaceBinary(selfupdate.ReplaceBinaryParams{
		Release:    srv.release(),
		BinaryPath: target,
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Timeout:    5 * time.Second,
	})
	if !errors.Is(err, domain.ErrChecksumMismatch) {
		t.Fatalf("err = %v, want ErrChecksumMismatch", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "OLD BINARY" {
		t.Fatalf("binary content = %q, want the original %q untouched", got, "OLD BINARY")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("directory holds %d entries, want 1 — a temp file leaked", len(entries))
	}
}

func TestReplaceBinaryMissingAssetForPlatform(t *testing.T) {
	archive := buildArchive(t, "NEW BINARY")
	srv := newReleaseServer(t, archive, sha256Hex(archive))
	defer srv.Close()

	_, target := seedBinary(t)

	err := selfupdate.ReplaceBinary(selfupdate.ReplaceBinaryParams{
		Release:    srv.release(),
		BinaryPath: target,
		GOOS:       "linux",
		GOARCH:     "amd64",
		Timeout:    5 * time.Second,
	})
	if !errors.Is(err, domain.ErrReleaseAssetMissing) {
		t.Fatalf("err = %v, want ErrReleaseAssetMissing", err)
	}
}

func TestReplaceBinaryReadOnlyDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}

	archive := buildArchive(t, "NEW BINARY")
	srv := newReleaseServer(t, archive, sha256Hex(archive))
	defer srv.Close()

	dir, target := seedBinary(t)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	err := selfupdate.ReplaceBinary(selfupdate.ReplaceBinaryParams{
		Release:    srv.release(),
		BinaryPath: target,
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Timeout:    5 * time.Second,
	})
	if !errors.Is(err, domain.ErrUpgradeNotWritable) {
		t.Fatalf("err = %v, want ErrUpgradeNotWritable", err)
	}
}
