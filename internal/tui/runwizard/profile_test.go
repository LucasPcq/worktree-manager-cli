package runwizard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

func testConfig() domain.RunConfig {
	return domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "build", Kind: domain.JobKindTask, Cmd: "make build"},
			{Name: "db", Kind: domain.JobKindService, Cmd: "docker compose up db"},
			{Name: "server", Kind: domain.JobKindService, Cmd: "go run ."},
		},
	}
}

func TestBuildReorderItemsPreservesOrderAndLabels(t *testing.T) {
	got := buildReorderItems(testConfig(), []string{"server", "build"})
	if len(got) != 2 {
		t.Fatalf("expected 2 items, got %d", len(got))
	}
	if got[0].Value != "server" || got[1].Value != "build" {
		t.Fatalf("expected order [server, build], got [%s, %s]", got[0].Value, got[1].Value)
	}
	if got[0].Label != "server (service)" {
		t.Fatalf("expected label \"server (service)\", got %q", got[0].Label)
	}
}

func TestBuildReorderItemsSkipsUnknown(t *testing.T) {
	got := buildReorderItems(testConfig(), []string{"build", "ghost", "db"})
	if len(got) != 2 {
		t.Fatalf("expected unknown name dropped, got %d items", len(got))
	}
	if got[0].Value != "build" || got[1].Value != "db" {
		t.Fatalf("expected [build, db], got [%s, %s]", got[0].Value, got[1].Value)
	}
}

func TestSelectedJobNamesReadsJobsStep(t *testing.T) {
	jobs := components.NewMultiSelect(components.NewMultiSelectParams{
		Items: buildJobItems(testConfig(), []string{"build", "server"}),
	})
	prev := make([]components.Step, stepProfileOrder)
	prev[stepProfileJobs] = components.Step{Model: jobs}

	got := selectedJobNames(prev)
	if strings.Join(got, ",") != "build,server" {
		t.Fatalf("expected build,server, got %v", got)
	}
}

// drive feeds key messages to the wizard and returns the updated model.
func drive(t *testing.T, m components.WizardModel, msgs ...tea.Msg) components.WizardModel {
	t.Helper()
	for _, msg := range msgs {
		next, _ := m.Update(msg)
		w, ok := next.(components.WizardModel)
		if !ok {
			t.Fatalf("expected WizardModel, got %T", next)
		}
		m = w
	}
	return m
}

func keyEnter() tea.KeyMsg     { return tea.KeyMsg{Type: tea.KeyEnter} }
func keySpace() tea.KeyMsg     { return tea.KeyMsg{Type: tea.KeySpace} }
func keyDown() tea.KeyMsg      { return tea.KeyMsg{Type: tea.KeyDown} }
func keyUp() tea.KeyMsg        { return tea.KeyMsg{Type: tea.KeyUp} }
func keyShiftDown() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyShiftDown} }

// TestProfileWizardOrderDrivesFinalJobOrder drives the full wizard: select two
// jobs, reorder them, then confirm extractProfile returns the reordered slice —
// proving the Order step (not the Jobs selection order) defines the result.
func TestProfileWizardOrderDrivesFinalJobOrder(t *testing.T) {
	wiz := components.NewWizard(buildProfileSteps(ProfileWizardParams{
		Existing: testConfig(),
		Initial:  domain.ProfileConfig{Name: "dev"},
	}))

	wiz = drive(t, wiz,
		keyEnter(), // Name: "dev" is seeded as default, confirm

		keySpace(),         // Jobs: select "build" (index 0)
		keyDown(), keyDown(), // move cursor to "server" (index 2)
		keySpace(),         // select "server"
		keyEnter(),         // confirm Jobs -> selection order [build, server]

		keyShiftDown(), // Order: move "build" below "server" -> [server, build]
		keyEnter(),     // confirm Order

		keyUp(),    // Default: move from "No" to "Yes"
		keyEnter(), // confirm Default -> true
	)

	if !wiz.Done() {
		t.Fatalf("expected wizard done after all steps")
	}

	got := extractProfile(wiz.Steps())
	if got.Name != "dev" {
		t.Fatalf("expected name dev, got %q", got.Name)
	}
	if strings.Join(got.Jobs, ",") != "server,build" {
		t.Fatalf("expected jobs [server, build], got %v", got.Jobs)
	}
	if !got.Default {
		t.Fatalf("expected default true")
	}
}
