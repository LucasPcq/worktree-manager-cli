package detect

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// PnpmWorkspacePackages parses pnpm-workspace.yaml and resolves the package patterns
// to actual directories. Returns nil if pnpm-workspace.yaml doesn't exist.
func PnpmWorkspacePackages(projectDir string) []string {
	wsPath := filepath.Join(projectDir, "pnpm-workspace.yaml")
	data, err := os.ReadFile(wsPath)
	if err != nil {
		return nil
	}

	patterns := parsePnpmWorkspace(string(data))
	var packages []string

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(projectDir, pattern))
		if err != nil {
			continue
		}
		for _, m := range matches {
			info, statErr := os.Stat(m)
			if statErr != nil || !info.IsDir() {
				continue
			}
			rel, relErr := filepath.Rel(projectDir, m)
			if relErr == nil {
				packages = append(packages, rel)
			}
		}
	}

	sort.Strings(packages)
	return packages
}

// parsePnpmWorkspace extracts package patterns from pnpm-workspace.yaml content.
// Handles the simple format: packages:\n  - "pattern"\n  - 'pattern'\n  - pattern
func parsePnpmWorkspace(content string) []string {
	var patterns []string
	inPackages := false

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)

		if trimmed == "packages:" {
			inPackages = true
			continue
		}

		if inPackages {
			if !strings.HasPrefix(trimmed, "- ") {
				break
			}
			pattern := strings.TrimPrefix(trimmed, "- ")
			pattern = strings.Trim(pattern, "\"'")
			if pattern != "" && !strings.HasPrefix(pattern, "!") {
				patterns = append(patterns, pattern)
			}
		}
	}

	return patterns
}
