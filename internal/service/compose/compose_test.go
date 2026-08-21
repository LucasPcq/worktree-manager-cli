package compose

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

const richCompose = `# une stack de dev
x-common: &common
  restart: unless-stopped

services:
  postgres:                    # la base
    image: postgres:16-alpine
    ports:
      - "${DB_PORT:-5432}:5432"
      - 9187:9187
  redis:
    ports: ['6379:6379']
  mailhog:
    ports:
      - "127.0.0.1:8025:8025"
      - target: 1025
        published: 1025
  edge:
    ports:
      - "3000-3005:3000-3005"
      - "9999"
  novars:
    image: busybox
`

func writeCompose(t *testing.T, content string) (dir, file string) {
	t.Helper()
	dir = t.TempDir()
	file = "docker-compose.yml"
	if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return dir, file
}

func findBinding(t *testing.T, scan domain.ComposeScan, service string, container int) domain.ComposePortBinding {
	t.Helper()
	for _, b := range scan.Bindings {
		if b.Service == service && b.Container == container {
			return b
		}
	}
	t.Fatalf("no binding for %s/%d in %+v", service, container, scan.Bindings)
	return domain.ComposePortBinding{}
}

func TestScanClassifiesEveryMappingForm(t *testing.T) {
	dir, file := writeCompose(t, richCompose)
	scan := Scan(ScanParams{ProjectDir: dir, File: file})
	if scan.Err != "" {
		t.Fatalf("scan: %s", scan.Err)
	}

	templated := findBinding(t, scan, "postgres", 5432)
	if templated.Status != domain.ComposePortTemplated || templated.Var != "DB_PORT" || templated.Base != 5432 {
		t.Errorf("postgres/5432 = %+v, want templated DB_PORT=5432", templated)
	}
	if templated.Token != `"${DB_PORT:-5432}:5432"` {
		t.Errorf("token = %s, want the quoted source scalar", templated.Token)
	}

	frozen := findBinding(t, scan, "postgres", 9187)
	if frozen.Status != domain.ComposePortFrozen || frozen.Var != "POSTGRES_PORT" || frozen.Token != "9187:9187" {
		t.Errorf("postgres/9187 = %+v, want frozen POSTGRES_PORT with an unquoted token", frozen)
	}

	flow := findBinding(t, scan, "redis", 6379)
	if flow.Status != domain.ComposePortFrozen || flow.Var != "REDIS_PORT" || flow.Token != "'6379:6379'" {
		t.Errorf("redis/6379 = %+v, want frozen REDIS_PORT with a single-quoted token", flow)
	}

	withIP := findBinding(t, scan, "mailhog", 8025)
	if withIP.Replacement != `"127.0.0.1:${MAILHOG_PORT:-8025}:8025"` {
		t.Errorf("mailhog/8025 replacement = %s, want the host IP kept", withIP.Replacement)
	}

	long := findBinding(t, scan, "mailhog", 1025)
	if long.Status != domain.ComposePortFrozen || long.Var != "MAILHOG_PORT_1025" || long.Token != "1025" {
		t.Errorf("mailhog/1025 = %+v, want frozen MAILHOG_PORT_1025 on the published scalar", long)
	}
	if long.Replacement != `"${MAILHOG_PORT_1025:-1025}"` {
		t.Errorf("mailhog/1025 replacement = %s", long.Replacement)
	}

	var unsupportedReasons []string
	for _, b := range scan.Bindings {
		if b.Service == "edge" {
			unsupportedReasons = append(unsupportedReasons, b.Reason)
			if b.Status != domain.ComposePortUnsupported {
				t.Errorf("edge binding %+v must be unsupported", b)
			}
			if b.Replacement != "" {
				t.Errorf("an unsupported binding is never rewritten, got %q", b.Replacement)
			}
		}
	}
	if len(unsupportedReasons) != 2 {
		t.Fatalf("edge must report both its mappings, got %v", unsupportedReasons)
	}
	if !strings.Contains(unsupportedReasons[0], "range") || !strings.Contains(unsupportedReasons[1], "no host port") {
		t.Errorf("reasons = %v", unsupportedReasons)
	}
}

func TestScanIgnoresServicesWithoutPorts(t *testing.T) {
	dir, file := writeCompose(t, richCompose)
	scan := Scan(ScanParams{ProjectDir: dir, File: file})
	for _, b := range scan.Bindings {
		if b.Service == "novars" {
			t.Errorf("a service declaring no ports must produce nothing, got %+v", b)
		}
	}
}

func TestScanReportsAnAliasedPortsList(t *testing.T) {
	dir, file := writeCompose(t, `x-ports: &shared
  - "5432:5432"

services:
  a:
    ports: *shared
`)
	scan := Scan(ScanParams{ProjectDir: dir, File: file})
	if len(scan.Bindings) != 1 || scan.Bindings[0].Status != domain.ComposePortUnsupported {
		t.Fatalf("got %+v, want one unsupported binding", scan.Bindings)
	}
	if !strings.Contains(scan.Bindings[0].Reason, "alias") {
		t.Errorf("reason = %q, want it to name the alias", scan.Bindings[0].Reason)
	}
}

func TestScanReportsUnreadableAndMissingFiles(t *testing.T) {
	dir := t.TempDir()
	if missing := Scan(ScanParams{ProjectDir: dir, File: "nope.yml"}); missing.Err == "" {
		t.Error("a missing file must come back with Err set, not empty bindings")
	}

	dir, file := writeCompose(t, "services:\n  a:\n   ports: [\n")
	if broken := Scan(ScanParams{ProjectDir: dir, File: file}); broken.Err == "" {
		t.Error("unparseable YAML must come back with Err set")
	}
}

func TestPatchRewritesOnlyThePortLines(t *testing.T) {
	dir, file := writeCompose(t, richCompose)
	scan := Scan(ScanParams{ProjectDir: dir, File: file})

	var frozen []domain.ComposePortBinding
	for _, b := range scan.Bindings {
		if b.Status == domain.ComposePortFrozen {
			frozen = append(frozen, b)
		}
	}
	if len(frozen) != 4 {
		t.Fatalf("want 4 frozen mappings, got %d", len(frozen))
	}

	if err := Patch(PatchParams{ProjectDir: dir, File: file, Bindings: frozen}); err != nil {
		t.Fatalf("patch: %v", err)
	}

	patched, err := os.ReadFile(filepath.Join(dir, file))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	got := string(patched)

	for _, want := range []string{
		`- "${POSTGRES_PORT:-9187}:9187"`,
		`ports: ["${REDIS_PORT:-6379}:6379"]`,
		`- "127.0.0.1:${MAILHOG_PORT:-8025}:8025"`,
		`published: "${MAILHOG_PORT_1025:-1025}"`,
		`- "${DB_PORT:-5432}:5432"`,
		"# une stack de dev",
		"  postgres:                    # la base",
		"x-common: &common",
		`- "3000-3005:3000-3005"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("patched file is missing %q:\n%s", want, got)
		}
	}

	if before, after := strings.Count(richCompose, "\n"), strings.Count(got, "\n"); before != after {
		t.Errorf("line count changed: %d → %d", before, after)
	}
}

func TestPatchRefusesAFileChangedSinceTheScan(t *testing.T) {
	dir, file := writeCompose(t, richCompose)
	scan := Scan(ScanParams{ProjectDir: dir, File: file})

	if err := os.WriteFile(filepath.Join(dir, file), []byte("services:\n  postgres:\n    ports:\n      - \"1234:1234\"\n"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	var frozen []domain.ComposePortBinding
	for _, b := range scan.Bindings {
		if b.Status == domain.ComposePortFrozen {
			frozen = append(frozen, b)
		}
	}

	err := Patch(PatchParams{ProjectDir: dir, File: file, Bindings: frozen})
	if err == nil {
		t.Fatal("patching a file that moved under us must fail")
	}

	after, readErr := os.ReadFile(filepath.Join(dir, file))
	if readErr != nil {
		t.Fatalf("read back: %v", readErr)
	}
	if !strings.Contains(string(after), `"1234:1234"`) {
		t.Errorf("the file must be left untouched on failure, got:\n%s", after)
	}
}

func TestScanAllKeysByRelativePath(t *testing.T) {
	dir, file := writeCompose(t, richCompose)
	scans := ScanAll(dir, []string{file, "absent.yml"})
	if len(scans) != 2 {
		t.Fatalf("got %d scans, want 2", len(scans))
	}
	if len(scans[file].Bindings) == 0 {
		t.Error("the existing file must carry its bindings")
	}
	if scans["absent.yml"].Err == "" {
		t.Error("the missing file must carry its error")
	}
	if ScanAll(dir, nil) != nil {
		t.Error("no file means no map")
	}
}
