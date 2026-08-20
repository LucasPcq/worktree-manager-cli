// Package socktest hands tests a Unix socket path short enough to bind.
package socktest

import (
	"os"
	"path/filepath"
	"testing"
)

// shortTempBase is used instead of t.TempDir(), which embeds the test name
// under /var/folders on macOS and overruns sun_path (104 bytes), failing the
// bind with EINVAL.
const shortTempBase = "/tmp"

// Path returns a daemon socket path under a directory removed when the test ends.
func Path(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp(shortTempBase, "wtm")
	if err != nil {
		t.Fatalf("socket dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, "wtm.sock")
}
