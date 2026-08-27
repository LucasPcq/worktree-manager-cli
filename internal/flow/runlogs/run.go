package runlogs

import (
	"context"
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type Outcome struct {
	// Profile names the profile this run started, empty when the caller started
	// a job list of its own. A surface that cannot say which profile it just
	// brought up leaves the reader to guess between several (LUC-208).
	Profile string
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
	// Probes is what the port check observed, empty when no Prober was installed
	// or when the sequence aborted before there was anything to check.
	Probes []domain.PortProbe
}

func (o Outcome) Aborted() bool { return o.Failed != "" }

// Recorded reports that the run got far enough to have an account of itself. A
// zero Outcome is a run that never gave one — a surface detached from before
// the first job, or a refusal ahead of it — and it must not replace the account
// a surface already has.
func (o Outcome) Recorded() bool { return o.Steps > 0 || o.Aborted() }

type RunParams struct {
	Service Service
	Sink    Sink
	Jobs    []domain.JobConfig
	// Profile is what the surface resolved Jobs from, carried through to the
	// Outcome so a recap can name it.
	Profile string
	WorkDir string
	LogDir  string
	// Env is what every job in this run learns about the worktree it belongs to,
	// resolved by the surface — the only side that can ask git.
	Env map[string]string
	// Prober checks that the ports the jobs declared were actually bound. Nil
	// skips the check entirely, which is what --no-probe and a zero budget do.
	Prober Prober
	// Project names the repository, the last segment of every route. Empty
	// leaves the jobs on their own ports.
	Project string
	// ProxyPort is where the proxy serves those routes. Zero means it is off,
	// and a job's URL is then its own address.
	ProxyPort int
	// NextConfig reads a job's next.config.*, so the run can say what a Next
	// project is missing before its own name reaches it. Nil skips the check.
	NextConfig NextConfigLookup
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
		ctx:        ctx,
		service:    params.Service,
		sink:       params.Sink,
		jobs:       params.Jobs,
		profile:    params.Profile,
		workDir:    params.WorkDir,
		logDir:     params.LogDir,
		env:        params.Env,
		prober:     params.Prober,
		project:    params.Project,
		proxyPort:  params.ProxyPort,
		nextConfig: params.NextConfig,
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
	profile string
	workDir string
	logDir  string
	env     map[string]string
	prober  Prober
	// project and proxyPort together decide whether a job's URL is its name or
	// its port; both come from the surface, which is the side that reads config.
	project    string
	proxyPort  int
	nextConfig NextConfigLookup

	// probeTargets are the started services that declared ports, kept in start
	// order so the check runs once, at the end, when everything is up.
	probeTargets []probeTarget
	results      []domain.JobActionResult
	started      []string
	completed    []string
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
		host := rules.RouteHost(rules.RouteHostParams{
			Job:      job,
			Worktree: r.env[domain.EnvWorktree],
			Project:  r.project,
		})

		result, err := r.service.Start(r.ctx, StartRequest{
			Job:       job,
			WorkDir:   r.workDir,
			LogDir:    r.logDir,
			Env:       r.env,
			RouteHost: host,
			OnOutput: func(chunk []byte) {
				r.captured = append(r.captured, chunk...)
				r.emit(Event{Phase: PhaseOutput, Job: job.Name, Kind: job.Kind, Step: i + 1, Chunk: chunk})
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
			r.emit(Event{Phase: PhaseDone, Job: job.Name, Step: i + 1, Ports: result.Ports, URL: r.jobURL(job, result.Ports, host)})
			continue
		}

		r.started = append(r.started, job.Name)
		r.results = append(r.results, domain.JobActionResult{Name: job.Name, Status: domain.JobActionStarted})
		if rules.ShouldProbeJob(job.Kind, result.Ports) {
			r.probeTargets = append(r.probeTargets, probeTarget{job: job.Name, resolved: result.Ports})
		}
		r.emit(Event{Phase: PhaseStarted, Job: job.Name, Step: i + 1, AlreadyRunning: alreadyRunning, Ports: result.Ports, URL: r.jobURL(job, result.Ports, host), DevOrigins: r.devOrigins(job, host)})
	}

	probes := r.probe()

	outcome := r.outcome()
	outcome.Probes = probes
	r.emit(Event{Phase: PhaseReady, Outcome: outcome})
	return outcome
}

// devOrigins is only ever asked when the proxy actually serves the job: under
// its own port, Next has nothing to allow.
func (r *runner) devOrigins(job domain.JobConfig, host string) []domain.DevOriginFix {
	if r.nextConfig == nil || host == "" || r.proxyPort == 0 {
		return nil
	}
	path, source := r.nextConfig(job)
	if !rules.NeedsDevOrigins(rules.NeedsDevOriginsParams{Job: job, ConfigSource: source}) {
		return nil
	}
	return []domain.DevOriginFix{{
		Job:    job.Name,
		Config: path,
		Line:   fmt.Sprintf(domain.DevOriginsFixFmt, job.Name, domain.ProxyTLD, r.proxyPort, path),
	}}
}

func (r *runner) jobURL(job domain.JobConfig, ports map[string]int, host string) string {
	return rules.JobURL(rules.JobURLParams{Job: job, Ports: ports, Host: host, ProxyPort: r.proxyPort})
}

type probeTarget struct {
	job string
	// resolved is what the daemon answered it bound, base plus offset. Read back
	// rather than recomputed, so the number injected and the number checked
	// cannot drift apart.
	resolved map[string]int
}

// probe checks every started service's declared ports in one pass, once they
// are all up: a service started first would otherwise be checked before the one
// it waits on has even been asked to start.
func (r *runner) probe() []domain.PortProbe {
	if r.prober == nil || len(r.probeTargets) == 0 || r.ctx.Err() != nil {
		return nil
	}

	offset := rules.PortOffsetFromEnv(r.env)
	var wanted []int
	for _, target := range r.probeTargets {
		wanted = append(wanted, rules.PortsToDial(rules.DiagnosePortProbesParams{
			Resolved: target.resolved,
			Offset:   offset,
		})...)
	}

	listening := r.prober.Listening(r.ctx, rules.DedupePorts(wanted), func(answered map[int]bool) bool {
		return r.settled(answered, offset)
	})

	var probes []domain.PortProbe
	for _, target := range r.probeTargets {
		found := rules.DiagnosePortProbes(rules.DiagnosePortProbesParams{
			Job:       target.job,
			Resolved:  target.resolved,
			Listening: listening,
			Offset:    offset,
		})
		r.emit(Event{Phase: PhaseProbed, Job: target.job, Probes: found})
		probes = append(probes, found...)
	}
	return probes
}

// settled ends the poll as soon as every declared port answers — the healthy
// case, which must not wait out the budget.
func (r *runner) settled(answered map[int]bool, offset int) bool {
	for _, target := range r.probeTargets {
		if !rules.AllPortsListening(rules.DiagnosePortProbes(rules.DiagnosePortProbesParams{
			Job:       target.job,
			Resolved:  target.resolved,
			Listening: answered,
			Offset:    offset,
		})) {
			return false
		}
	}
	return true
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
		Profile:   r.profile,
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
