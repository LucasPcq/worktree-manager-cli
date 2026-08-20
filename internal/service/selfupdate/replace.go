package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

const (
	maxArchiveBytes = 100 << 20
	maxRedirects    = 10
	schemeHTTPS     = "https"
)

// allowInsecureDownload is flipped only by export_test.go, so the tests can drive
// a plaintext httptest server while a release binary refuses one.
var allowInsecureDownload bool

type ReplaceBinaryParams struct {
	Release    domain.ReleaseInfo
	BinaryPath string
	GOOS       string
	GOARCH     string
	UserAgent  string
	Timeout    time.Duration
}

func ReplaceBinary(params ReplaceBinaryParams) error {
	assetName := rules.ReleaseAssetName(rules.ReleaseAssetNameParams{
		Version: params.Release.Version,
		GOOS:    params.GOOS,
		GOARCH:  params.GOARCH,
	})

	assetURL, ok := findAsset(params.Release, assetName)
	if !ok {
		return fmt.Errorf("%w: %s", domain.ErrReleaseAssetMissing, assetName)
	}

	checksumsURL, ok := findAsset(params.Release, domain.ChecksumsFileName)
	if !ok {
		return fmt.Errorf("%w: %s", domain.ErrReleaseAssetMissing, domain.ChecksumsFileName)
	}

	client := &http.Client{
		Timeout: params.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme != schemeHTTPS {
				return fmt.Errorf("refusing redirect to %s", req.URL.Scheme)
			}
			if len(via) >= maxRedirects {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	archive, err := download(client, assetURL, params.UserAgent)
	if err != nil {
		return err
	}

	manifest, err := download(client, checksumsURL, params.UserAgent)
	if err != nil {
		return err
	}

	want, ok := checksumFor(string(manifest), assetName)
	if !ok {
		return fmt.Errorf("%w: %s absent from %s", domain.ErrChecksumMismatch, assetName, domain.ChecksumsFileName)
	}

	sum := sha256.Sum256(archive)
	if hex.EncodeToString(sum[:]) != want {
		return domain.ErrChecksumMismatch
	}

	binary, err := extractBinary(archive)
	if err != nil {
		return err
	}

	return swap(params.BinaryPath, binary)
}

func findAsset(release domain.ReleaseInfo, name string) (string, bool) {
	for _, asset := range release.Assets {
		if asset.Name == name {
			return asset.URL, true
		}
	}

	return "", false
}

func download(client *http.Client, rawURL string, userAgent string) ([]byte, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	// The URLs come from the API payload; a release that names a plaintext host
	// would otherwise be fetched over it.
	if parsed.Scheme != schemeHTTPS && !allowInsecureDownload {
		return nil, fmt.Errorf("refusing to download over %q", parsed.Scheme)
	}

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", rawURL, resp.Status)
	}

	return io.ReadAll(io.LimitReader(resp.Body, maxArchiveBytes))
}

func checksumFor(manifest string, assetName string) (string, bool) {
	for _, line := range strings.Split(manifest, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == assetName {
			return fields[0], true
		}
	}

	return "", false
}

func extractBinary(archive []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read archive: %w", err)
		}
		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != domain.AppName {
			continue
		}

		binary, err := io.ReadAll(io.LimitReader(tr, maxArchiveBytes))
		if err != nil {
			return nil, fmt.Errorf("read %s from archive: %w", domain.AppName, err)
		}
		// A truncated entry would otherwise be renamed over the user's binary as
		// a short or zero-byte file, with no working wtm left to recover with.
		if int64(len(binary)) != header.Size {
			return nil, fmt.Errorf("%w: %s entry is %d bytes, declared %d", domain.ErrChecksumMismatch, domain.AppName, len(binary), header.Size)
		}
		if len(binary) == 0 {
			return nil, fmt.Errorf("%w: %s entry is empty", domain.ErrChecksumMismatch, domain.AppName)
		}

		return binary, nil
	}

	return nil, fmt.Errorf("%w: no %s entry in the archive", domain.ErrReleaseAssetMissing, domain.AppName)
}

// swap writes the new binary next to the current one — same filesystem, so the
// rename is atomic — then renames it over the target.
func swap(binaryPath string, content []byte) error {
	dir := filepath.Dir(binaryPath)

	// Preserve the mode the user installed with: a deliberately private 0700
	// binary must not be widened by an upgrade.
	mode := os.FileMode(0o755)
	if info, err := os.Stat(binaryPath); err == nil {
		mode = info.Mode().Perm()
	}

	tmp, err := os.CreateTemp(dir, "."+domain.AppName+"-upgrade-*")
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return domain.ErrUpgradeNotWritable
		}
		return err
	}
	tmpPath := tmp.Name()

	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	// Flushed before the rename: the temp+rename dance exists to make the swap
	// crash-safe, which a rename over unflushed data does not achieve.
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpPath, binaryPath); err != nil {
		cleanup()
		if errors.Is(err, os.ErrPermission) {
			return domain.ErrUpgradeNotWritable
		}
		return err
	}

	return nil
}
