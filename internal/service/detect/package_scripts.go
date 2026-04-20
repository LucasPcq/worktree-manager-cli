package detect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// PackageJSONScripts returns all package.json scripts under projectDir,
// including pnpm workspace packages when pnpm-workspace.yaml is present.
// Results are ordered: root scripts first (alphabetically), then workspace
// packages (alphabetically by dir, then by script name within each package).
// Returns nil if no package.json is found at the project root.
func PackageJSONScripts(projectDir string) []domain.PackageScript {
	root := readPackageScripts(projectDir, "")
	if root == nil {
		return nil
	}

	scripts := append([]domain.PackageScript{}, root...)

	for _, wsDir := range PnpmWorkspacePackages(projectDir) {
		ws := readPackageScripts(filepath.Join(projectDir, wsDir), wsDir)
		scripts = append(scripts, ws...)
	}

	return scripts
}

type packageJSONFile struct {
	Name    string            `json:"name"`
	Scripts map[string]string `json:"scripts"`
}

func readPackageScripts(dir, workspace string) []domain.PackageScript {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return nil
	}

	var pkg packageJSONFile
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}

	pkgName := stripScope(pkg.Name)
	if pkgName == "" {
		pkgName = filepath.Base(dir)
	}

	names := make([]string, 0, len(pkg.Scripts))
	for name := range pkg.Scripts {
		names = append(names, name)
	}
	sort.Strings(names)

	scripts := make([]domain.PackageScript, 0, len(names))
	for _, name := range names {
		scripts = append(scripts, domain.PackageScript{
			Name:      name,
			Cmd:       pkg.Scripts[name],
			Workspace: workspace,
			PkgName:   pkgName,
		})
	}

	return scripts
}

// stripScope removes an npm scope prefix from a package name.
// "@acme/web" → "web", "my-app" → "my-app".
func stripScope(name string) string {
	if !strings.HasPrefix(name, "@") {
		return name
	}
	parts := strings.SplitN(name, "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return name
}
