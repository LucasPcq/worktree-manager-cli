package up

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/seam"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
)

const here = "/wt/here"

func running(dirs ...string) []domain.JobInfo {
	jobs := make([]domain.JobInfo, 0, len(dirs))
	for i, dir := range dirs {
		jobs = append(jobs, domain.JobInfo{
			Name:    []string{"web", "db", "api"}[i%3],
			WorkDir: dir,
			Status:  domain.JobStatusRunning,
		})
	}
	return jobs
}

func flowWith(request Request, jobs []domain.JobInfo) *upFlow {
	if request.Cwd == "" {
		request.Cwd = here
	}
	return &upFlow{request: request, jobs: jobs}
}

func concurrencyStepOf(f *upFlow) flow.Step { return f.concurrencyStep() }

func TestConcurrencyIsNotAskedWhenNothingRunsElsewhere(t *testing.T) {
	f := flowWith(Request{}, running(here))

	skip, reason := concurrencyStepOf(f).Skip(flow.Answers{})
	if !skip || reason != domain.RunConcurrencySkipAlone {
		t.Errorf("Skip = (%v, %q), want the step skipped for want of a neighbour", skip, reason)
	}
}

// A worktree's own jobs are never a reason to ask: `run up X` must not offer to
// stop X's own services.
func TestConcurrencyMeasuresAgainstTheTargetNotTheCurrentDirectory(t *testing.T) {
	f := flowWith(Request{Cwd: here}, running("/wt/other"))
	answers := flow.NewAnswers(map[string]string{"run.worktree": "/wt/other"})

	if skip, _ := concurrencyStepOf(f).Skip(answers); !skip {
		t.Error("the step was asked about the very worktree the run targets")
	}
}

func TestConcurrencyIsAskedOnceThenNeverAgain(t *testing.T) {
	asked := flowWith(Request{}, running(here, "/wt/other"))
	if skip, _ := concurrencyStepOf(asked).Skip(flow.Answers{}); skip {
		t.Fatal("the step was skipped although another worktree is running jobs")
	}

	settled := flowWith(Request{Config: domain.RunConfig{Concurrency: domain.ConcurrencyExclusive}}, running(here, "/wt/other"))
	skip, reason := concurrencyStepOf(settled).Skip(flow.Answers{})
	if !skip || reason != domain.RunConcurrencySkipSettled {
		t.Errorf("Skip = (%v, %q), want the config to have settled it", skip, reason)
	}
}

func TestConcurrencyFlagsAnswerWithoutAsking(t *testing.T) {
	for _, tc := range []struct {
		name    string
		request Request
		want    domain.Concurrency
	}{
		{"--exclusive", Request{Exclusive: true}, domain.ConcurrencyExclusive},
		{"--parallel", Request{Parallel: true}, domain.ConcurrencyParallel},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := flowWith(tc.request, running(here, "/wt/other"))
			step := concurrencyStepOf(f)

			if skip, _ := step.Skip(flow.Answers{}); !skip {
				t.Error("the step was asked although a flag answered it")
			}
			answer, err := step.Resolve(flow.Answers{})
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if concurrencyOf(answer.Value) != tc.want {
				t.Errorf("Resolve = %q, want %q", answer.Value, tc.want)
			}
		})
	}
}

// The safe default stops nothing: an unattended run must never tear down
// another worktree's services on its own initiative.
func TestConcurrencyResolvesToLeavingTheOthersAlone(t *testing.T) {
	f := flowWith(Request{}, running(here, "/wt/other"))

	answer, err := concurrencyStepOf(f).Resolve(flow.Answers{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if concurrencyOf(answer.Value) != domain.ConcurrencyParallel {
		t.Errorf("Resolve = %q, want the answer that stops nothing", answer.Value)
	}
}

func TestConcurrencyOffersFourAnswersAndNamesTheNeighbours(t *testing.T) {
	f := flowWith(Request{}, running(here, "/wt/other"))

	content, err := concurrencyStepOf(f).Build(flow.Answers{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	values := make([]string, 0, len(content.Options))
	for _, option := range content.Options {
		if option.Separator {
			continue
		}
		values = append(values, option.Value)
	}
	want := []string{answerParallel, answerParallelAlways, answerExclusive, answerExclusiveAlways}
	if strings.Join(values, ",") != strings.Join(want, ",") {
		t.Errorf("options = %v, want %v", values, want)
	}
	if !strings.Contains(content.Description, "other") {
		t.Errorf("description = %q, want it to name the worktree it is about", content.Description)
	}
}

func TestRememberWritesTheAnswerToRunTomlAndSaysSo(t *testing.T) {
	stateDir := t.TempDir()
	recorder := &flowtest.Recorder{}
	f := &upFlow{
		ctx:       flow.Context{StateDir: stateDir},
		request:   Request{Config: domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev"}}}},
		presenter: presenterOnly{recorder},
	}

	answers := flow.NewAnswers(map[string]string{KeyConcurrency: answerExclusiveAlways})
	if err := f.remember(answers); err != nil {
		t.Fatalf("remember: %v", err)
	}

	saved, err := runconfig.Load(stateDir)
	if err != nil {
		t.Fatalf("load run config: %v", err)
	}
	if saved.Concurrency != domain.ConcurrencyExclusive {
		t.Errorf("run.toml concurrency = %q, want %q", saved.Concurrency, domain.ConcurrencyExclusive)
	}
	if len(recorder.Statuses) != 1 || !strings.Contains(recorder.Statuses[0].Text, string(domain.ConcurrencyExclusive)) {
		t.Errorf("statuses = %v, want the write to be announced", recorder.Statuses)
	}
}

// A one-off answer changes nothing on disk: only the two "always" options do.
func TestAOneOffAnswerIsNotRemembered(t *testing.T) {
	stateDir := t.TempDir()
	f := &upFlow{ctx: flow.Context{StateDir: stateDir}, presenter: presenterOnly{&flowtest.Recorder{}}}

	if err := f.remember(flow.NewAnswers(map[string]string{KeyConcurrency: answerExclusive})); err != nil {
		t.Fatalf("remember: %v", err)
	}

	saved, err := runconfig.Load(stateDir)
	if err != nil {
		t.Fatalf("load run config: %v", err)
	}
	if saved.Concurrency != "" {
		t.Errorf("run.toml concurrency = %q, want it untouched", saved.Concurrency)
	}
}

// presenterOnly lends the up flow's Presenter the parts a step test needs; the
// hand-over to a surface belongs to the run, not to a question.
type presenterOnly struct{ *flowtest.Recorder }

func (presenterOnly) Sequence(seam.SequenceParams) (runlogs.Outcomes, error) {
	return runlogs.Outcomes{{}}, nil
}

// The regression that shipped: Skip short-circuits Resolve, so a settled step
// carries no value at all. Reading the answer alone turned every
// non-interactive --exclusive — and every project that had written
// concurrency = "exclusive" — into a parallel run that stopped nothing.
func TestASettledConcurrencyIsStillActedOn(t *testing.T) {
	cases := []struct {
		name    string
		request Request
		want    domain.Concurrency
	}{
		{"--exclusive", Request{Exclusive: true}, domain.ConcurrencyExclusive},
		{"--parallel", Request{Parallel: true}, domain.ConcurrencyParallel},
		{"config says exclusive", Request{Config: domain.RunConfig{Concurrency: domain.ConcurrencyExclusive}}, domain.ConcurrencyExclusive},
		{"config says parallel", Request{Config: domain.RunConfig{Concurrency: domain.ConcurrencyParallel}}, domain.ConcurrencyParallel},
		{"nobody answered", Request{}, domain.ConcurrencyParallel},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := flowWith(tc.request, running(here, "/wt/other"))

			// What an unattended run produces: Skip wins, and the step is recorded
			// as skipped with no value.
			answers, err := flow.Unattended{}.Ask(f.session())
			if err != nil {
				t.Fatalf("Ask: %v", err)
			}
			if answers.Answered(KeyConcurrency) {
				t.Fatal("the step was answered, so this test no longer covers the skipped path")
			}

			if got := f.concurrency(answers); got != tc.want {
				t.Errorf("concurrency = %q, want %q", got, tc.want)
			}
		})
	}
}

// A picker's answer outranks what the flags and the config would have resolved.
func TestAnAnsweredConcurrencyOutranksTheFallback(t *testing.T) {
	f := flowWith(Request{Parallel: true}, running(here, "/wt/other"))
	answers := flow.Answers{}.With(KeyConcurrency, flow.Answer{Value: answerExclusiveAlways, Asked: true})

	if got := f.concurrency(answers); got != domain.ConcurrencyExclusive {
		t.Errorf("concurrency = %q, want the answer that was actually given", got)
	}
}
