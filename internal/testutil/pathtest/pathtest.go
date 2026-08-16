// Package pathtest builds platform-appropriate paths for tests that assert on
// absoluteness semantics. filepath.IsAbs("/tmp/x") is true on POSIX and false on
// Windows, so fixtures hardcoding POSIX roots test the opposite branch there.
package pathtest

import (
	"path/filepath"
	"runtime"
)

// Root is the filesystem root for the current platform.
func Root() string {
	if runtime.GOOS == "windows" {
		return `C:\`
	}
	return "/"
}

// Abs joins segments onto the platform root: "/home/dev" on POSIX,
// `C:\home\dev` on Windows.
func Abs(segments ...string) string {
	return filepath.Join(append([]string{Root()}, segments...)...)
}
