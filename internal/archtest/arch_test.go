// Package archtest enforces the architecture membrane as a real, build-breaking
// test — not a doc line nobody reruns. It verifies the **transitive closure** of a
// layer, which a textual `grep "bubbletea" internal/cmd` cannot: the day an
// intermediate package (internal/output, internal/commands/shared, …) pulls tea,
// the grep stays empty while the layer's closure is nonetheless polluted. Here we
// walk the full `go list -deps` graph, so a transitive leak fails `go test ./...`.
package archtest

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

const module = "github.com/LucasPcq/wtm"

const (
	teaPkg      = "github.com/charmbracelet/bubbletea"
	lipglossPkg = "github.com/charmbracelet/lipgloss"
	cobraPkg    = "github.com/spf13/cobra"
)

// membrane rules: each layer's transitive import closure must exclude the listed
// packages (and their subpackages). These encode the hard rules of CLAUDE.md §9.
var membrane = []struct {
	layer     string // package pattern under the module (…/... for the whole subtree)
	forbidden []string
	why       string
}{
	{
		layer:     "internal/cmd/...",
		forbidden: []string{teaPkg},
		why:       "commands reach tea only through the wizard port + prompter/progress interfaces; their closure must exclude bubbletea",
	},
	{
		layer:     "internal/service/...",
		forbidden: []string{teaPkg, lipglossPkg, cobraPkg},
		why:       "the business layer must compile and test without cobra/bubbletea/lipgloss",
	},
	{
		layer:     "internal/rules/...",
		forbidden: []string{teaPkg, lipglossPkg, cobraPkg},
		why:       "pure rules import only stdlib + domain",
	},
	{
		layer:     "internal/domain/...",
		forbidden: []string{teaPkg, lipglossPkg, cobraPkg},
		why:       "domain holds types/errors/constants only",
	},
	{
		layer:     "internal/output/...",
		forbidden: []string{teaPkg},
		why:       "output renders (lipgloss) but must never drive the tea event loop",
	},
}

func TestArchitectureMembrane(t *testing.T) {
	for _, rule := range membrane {
		rule := rule
		t.Run(rule.layer, func(t *testing.T) {
			pattern := module + "/" + rule.layer
			imports := closureImports(t, pattern)

			for _, forbidden := range rule.forbidden {
				var bridges []string
				leaked := false
				for pkg, imps := range imports {
					if within(pkg, forbidden) {
						leaked = true
					}
					// Report the first-level offenders (closure packages that import
					// the forbidden package directly) so the failure is actionable.
					for _, imp := range imps {
						if within(imp, forbidden) {
							bridges = append(bridges, pkg)
							break
						}
					}
				}
				if leaked {
					t.Errorf("architecture violation: %s must not transitively import %s\n  reason: %s\n  bridges (packages in the closure importing it directly): %s\n  debug: go list -deps %s | grep %s",
						rule.layer, forbidden, rule.why, strings.Join(dedupe(bridges), ", "), pattern, forbidden)
				}
			}
		})
	}
}

// closureImports returns, for every package in the transitive closure of pattern,
// its direct imports — a single `go list -deps` pass over the production (non-test)
// import graph.
func closureImports(t *testing.T, pattern string) map[string][]string {
	t.Helper()
	cmd := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}|{{join .Imports \" \"}}", pattern)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list -deps %s: %v\n%s", pattern, err, stderr.String())
	}

	result := make(map[string][]string)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		path, imps, _ := strings.Cut(line, "|")
		result[path] = strings.Fields(imps)
	}
	return result
}

// within reports whether pkg is root or a subpackage of root.
func within(pkg, root string) bool {
	return pkg == root || strings.HasPrefix(pkg, root+"/")
}

func dedupe(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := in[:0]
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
