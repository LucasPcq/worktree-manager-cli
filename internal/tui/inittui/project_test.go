package inittui

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

func TestMoveToFront(t *testing.T) {
	items := []components.SelectItem{
		{Value: "example"},
		{Value: "main"},
		{Value: "parent"},
	}

	got := moveToFront(items, "parent")
	if got[0].Value != "parent" {
		t.Errorf("expected parent first, got %q", got[0].Value)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 items, got %d", len(got))
	}

	// Unknown / empty value leaves order untouched.
	if moveToFront(items, "")[0].Value != "example" {
		t.Error("empty value should not reorder")
	}
	if moveToFront(items, "nope")[0].Value != "example" {
		t.Error("unknown value should not reorder")
	}
}

func TestPrefillSelected(t *testing.T) {
	// Full init (nil prefill) uses the detection default.
	if !prefillSelected(nil, false, true) {
		t.Error("nil prefill should use the full-init default (true)")
	}
	if prefillSelected(nil, true, false) {
		t.Error("nil prefill should use the full-init default (false)")
	}

	// Re-init checks items already in the current config.
	if !prefillSelected(&SectionPrefill{}, true, false) {
		t.Error("configured item should be checked")
	}
	if prefillSelected(&SectionPrefill{}, false, true) {
		t.Error("non-configured item should be unchecked regardless of detection default")
	}
}

func composeDetection(bindings ...domain.ComposePortBinding) domain.InitDetectionResult {
	const file = "docker-compose.yml"
	for i := range bindings {
		bindings[i].File = file
	}
	return domain.InitDetectionResult{
		DockerComposeFiles: []string{file},
		DockerComposeCmd:   "docker compose",
		ComposeScans:       map[string]domain.ComposeScan{file: {File: file, Bindings: bindings}},
	}
}

// dockerSelection stands in for the completed compose-file multiselect the
// confirm step reads back.
func dockerSelection(files ...string) []components.Step {
	items := make([]components.MultiSelectItem, 0, len(files))
	for _, f := range files {
		items = append(items, components.MultiSelectItem{Label: f, Value: f, Selected: true})
	}
	return []components.Step{{
		Model: components.NewMultiSelect(components.NewMultiSelectParams{Items: items}),
	}}
}

func composeStepDecision(t *testing.T, detection domain.InitDetectionResult, authorized bool) (apply bool, reason string, params components.NewConfirmParams) {
	t.Helper()
	s := newStepSet()
	s.add(stepDocker, components.Step{})
	addComposePatchStep(s, addComposePatchStepParams{Detection: detection, Authorized: authorized})

	i := s.at(stepComposePatch)
	if i < 0 {
		t.Fatal("the confirm step was not registered")
	}
	step := s.steps[i]
	step.Build(dockerSelection("docker-compose.yml"))
	return !step.AutoSkip(components.WizardModel{}), step.SkipReason(), params
}

func TestComposePatchStepAsksOnlyWhenThereIsSomethingToRewrite(t *testing.T) {
	frozen := domain.ComposePortBinding{
		Service: "postgres", Status: domain.ComposePortFrozen,
		Var: "POSTGRES_PORT", Base: 5432,
		Token: `"5432:5432"`, Replacement: `"${POSTGRES_PORT:-5432}:5432"`,
	}

	apply, _, _ := composeStepDecision(t, composeDetection(frozen), false)
	if !apply {
		t.Error("a literal host port must raise the question")
	}

	templated := domain.ComposePortBinding{
		Service: "db", Status: domain.ComposePortTemplated, Var: "DB_PORT", Base: 5432,
	}
	apply, reason, _ := composeStepDecision(t, composeDetection(templated), false)
	if apply {
		t.Error("a file with nothing to rewrite must not raise the question")
	}
	if reason != "" {
		t.Errorf("an irrelevant step stays out of the recap, got %q", reason)
	}
}

func TestComposePatchStepStatesTheFlagInsteadOfAsking(t *testing.T) {
	frozen := domain.ComposePortBinding{
		Service: "postgres", Status: domain.ComposePortFrozen,
		Var: "POSTGRES_PORT", Base: 5432,
		Token: `"5432:5432"`, Replacement: `"${POSTGRES_PORT:-5432}:5432"`,
	}

	apply, reason, _ := composeStepDecision(t, composeDetection(frozen), true)
	if apply {
		t.Error("--patch-compose already answered the question")
	}
	if !strings.Contains(reason, domain.FlagPatchCompose) {
		t.Errorf("the flag must still appear in the recap, got %q", reason)
	}
}

func TestComposePatchStepIsAbsentWithoutAnyScan(t *testing.T) {
	s := newStepSet()
	s.add(stepDocker, components.Step{})
	addComposePatchStep(s, addComposePatchStepParams{Detection: domain.InitDetectionResult{}})

	if s.at(stepComposePatch) >= 0 {
		t.Error("no compose file means no step at all")
	}
}

func TestServicesStepPreselectsOnlyDevScripts(t *testing.T) {
	scripts := []domain.PackageScript{
		{Name: "dev", Workspace: "apps/web", PkgName: "web"},
		{Name: "build", Workspace: "apps/web", PkgName: "web"},
		{Name: "preview", Workspace: "apps/web", PkgName: "web"},
		{Name: "start", Workspace: "apps/api", PkgName: "api"},
	}

	items := scriptItems(scriptItemsParams{
		Scripts:        scripts,
		PackageManager: domain.PkgManagerPnpm,
	})

	if len(items) != 4 {
		t.Fatalf("tous les scripts restent proposés, got %d", len(items))
	}
	selected := map[string]bool{}
	for i, item := range items {
		selected[scripts[i].Name] = item.Selected
	}
	if !selected["dev"] {
		t.Error("dev doit être coché")
	}
	for _, name := range []string{"build", "preview", "start"} {
		if selected[name] {
			t.Errorf("%s ne doit pas être coché", name)
		}
	}
}

// L'étape propose tout coché : publier est additif — le job garde son port dans
// tous les cas — donc une mauvaise proposition coûte une case à décocher, jamais
// un run qui échoue.
func TestURLItemsOfferEveryCandidateChecked(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "web", Kind: domain.JobKindService, Ports: map[string]int{domain.PortNameDefault: 3000}},
		{Name: "api", Kind: domain.JobKindService, Ports: map[string]int{"API_PORT": 3100}},
		{Name: "cache", Kind: domain.JobKindService, Ports: map[string]int{"REDIS_PORT": 6379}},
	}}

	items := urlItemsFor(cfg)
	if len(items) != 2 {
		t.Fatalf("items = %d (%v), want web et api seulement", len(items), items)
	}
	for _, item := range items {
		if !item.Selected {
			t.Errorf("item %q n'est pas coché", item.Value)
		}
	}
	if items[0].Value != "web" || !strings.Contains(items[0].Label, domain.PortNameDefault) {
		t.Errorf("item[0] = %+v, want la valeur web et le port dans le libellé", items[0])
	}
}

// Sans service déclarant le port qu'il écoute, l'étape ne s'affiche pas du tout.
func TestURLStepIsAbsentWithoutACandidate(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "migrate", Kind: domain.JobKindTask, Ports: map[string]int{domain.PortNameDefault: 3000}},
	}}

	if items := urlItemsFor(cfg); len(items) != 0 {
		t.Errorf("items = %v, want aucun", items)
	}
}
