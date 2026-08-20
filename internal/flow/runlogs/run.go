package runlogs

import (
	"context"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type Outcome struct {
	Results []domain.JobActionResult
	// Started names the jobs left running when the sequence ended — after an
	// abort, the ones nothing tore down.
	Started []string
	// Completed names the tasks that ran to the end.
	Completed []string
	// NotStarted names the jobs the sequence never reached.
	NotStarted []string
	// Failed names the job that ended the sequence early, FailedStep placing it
	// among Steps. No Failed means every job was reached.
	Failed     string
	FailedStep int
	Steps      int
	// FailedOutput is what the job that ended the sequence had written, raw. A
	// surface that never showed it live — machine output, a CI log, an agent
	// reading JSON — has nothing else to say why the run stopped, and the
	// daemon's Reason alone ("task migrate failed: exit status 1") does not.
	FailedOutput []byte
	// FailedExitCode is the code the daemon reported for it, nil when it never
	// got as far as running.
	FailedExitCode *int
}

func (o Outcome) Aborted() bool { return o.Failed != "" }

type RunParams struct {
	Service Service
	Sink    Sink
	Jobs    []domain.JobConfig
	WorkDir string
	LogDir  string
}

// Run starts a profile's jobs in their declared order and reports each step to
// the Sink. A job that fails ends the sequence, leaving what is already running
// alone — the fix-and-retry loop keeps docker and databases warm — and that
// partial state is the Outcome, not an error: whether it exits non-zero, and
// what it prints, belongs to the surface.
//
// Cancelling ctx stops the reporting, not the jobs. A surface the user detached
// from is gone, and there is nobody left to emit to; what the daemon is running
// keeps running, the sequence stops where it stands, and the jobs it never
// reached come back as NotStarted with the context's error.
func Run(ctx context.Context, params RunParams) (Outcome, error) {
	if params.Service == nil {
		return Outcome{}, domain.ErrRunServiceRequired
	}

	r := &runner{
		ctx:     ctx,
		service: params.Service,
		sink:    params.Sink,
		jobs:    params.Jobs,
		workDir: params.WorkDir,
		logDir:  params.LogDir,
	}
	if r.sink == nil {
		r.sink = noSink{}
	}
	return r.run(), ctx.Err()
}

type runner struct {
	ctx     context.Context
	service Service
	sink    Sink
	jobs    []domain.JobConfig
	workDir string
	logDir  string

	results   []domain.JobActionResult
	started   []string
	completed []string
	// captured is what the job being started has written so far, kept only until
	// the next job starts: an abort is the one moment it has to be readable.
	captured []byte
}

func (r *runner) run() Outcome {
	for i := range r.jobs {
		if r.ctx.Err() != nil {
			return r.detached(i)
		}

		job := r.jobs[i]
		r.emit(Event{Phase: PhaseStarting, Job: job.Name, Step: i + 1})

		r.captured = nil
		result, err := r.service.Start(r.ctx, StartRequest{
			Job:     job,
			WorkDir: r.workDir,
			LogDir:  r.logDir,
			OnOutput: func(chunk []byte) {
				r.captured = append(r.captured, chunk...)
				r.emit(Event{Phase: PhaseOutput, Job: job.Name, Step: i + 1, Chunk: chunk})
			},
		})
		if err != nil {
			// A read the detach itself broke says nothing about the job.
			if r.ctx.Err() != nil {
				return r.detached(i)
			}
			return r.abort(abortParams{Index: i, Job: job, Reason: err.Error()})
		}

		// A repeat start of a service is what the caller asked for — the job is
		// up — so it counts as started. A task is a step to run, not a state to
		// reach: one the daemon refuses has not run.
		alreadyRunning := result.Refused && job.Kind != domain.JobKindTask && rules.IsAlreadyRunning(result.Message)
		if result.Refused && !alreadyRunning {
			return r.abort(abortParams{Index: i, Job: job, Reason: result.Message, ExitCode: result.ExitCode})
		}

		if job.Kind == domain.JobKindTask {
			r.completed = append(r.completed, job.Name)
			r.results = append(r.results, domain.JobActionResult{Name: job.Name, Status: domain.JobActionDone})
			r.emit(Event{Phase: PhaseDone, Job: job.Name, Step: i + 1})
			continue
		}

		r.started = append(r.started, job.Name)
		r.results = append(r.results, domain.JobActionResult{Name: job.Name, Status: domain.JobActionStarted})
		r.emit(Event{Phase: PhaseStarted, Job: job.Name, Step: i + 1, AlreadyRunning: alreadyRunning})
	}

	outcome := r.outcome()
	r.emit(Event{Phase: PhaseReady, Outcome: outcome})
	return outcome
}

type abortParams struct {
	Index    int
	Job      domain.JobConfig
	Reason   string
	ExitCode *int
}

func (r *runner) abort(params abortParams) Outcome {
	r.results = append(r.results, domain.JobActionResult{
		Name: params.Job.Name, Status: domain.JobActionError, Message: params.Reason,
	})
	r.emit(Event{Phase: PhaseFailed, Job: params.Job.Name, Step: params.Index + 1, Reason: params.Reason})

	outcome := r.outcome()
	outcome.Failed = params.Job.Name
	outcome.FailedStep = params.Index + 1
	outcome.NotStarted = jobNames(r.jobs[params.Index+1:])
	outcome.FailedOutput = r.captured
	outcome.FailedExitCode = params.ExitCode

	r.emit(Event{
		Phase: PhaseAborted, Job: params.Job.Name, Step: params.Index + 1,
		Reason: params.Reason, Outcome: outcome,
	})
	return outcome
}

// detached is the sequence stopping because nobody is watching any more. It is
// not an abort: nothing failed, nothing is torn down, and the jobs already up
// stay up.
func (r *runner) detached(index int) Outcome {
	outcome := r.outcome()
	outcome.NotStarted = jobNames(r.jobs[index:])
	return outcome
}

// emit drops what it is given once the run is detached: the surface it reported
// to is gone, and a phase it cannot show is noise on a dead channel.
func (r *runner) emit(event Event) {
	if r.ctx.Err() != nil {
		return
	}
	event.Steps = len(r.jobs)
	r.sink.Emit(event)
}

func (r *runner) outcome() Outcome {
	return Outcome{
		Results:   r.results,
		Started:   r.started,
		Completed: r.completed,
		Steps:     len(r.jobs),
	}
}

func jobNames(jobs []domain.JobConfig) []string {
	if len(jobs) == 0 {
		return nil
	}
	names := make([]string, len(jobs))
	for i, job := range jobs {
		names[i] = job.Name
	}
	return names
}
