package runlogs

import (
	"context"
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/detect"
)

type Outcome struct {
	// WorkDir and Worktree name the worktree this run acted on: its path as git
	// spells it, and the branch a recap shows. Both are empty for a run whose
	// caller never said.
	WorkDir  string
	Worktree string
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

// Outcomes is what a run over several worktrees concluded, one entry per
// worktree in the order they were selected. A run over one is an Outcomes of
// one: the surfaces read the arity rather than branching on a mode.
type Outcomes []Outcome

// Aborted reports that at least one worktree stopped short. The worktrees are
// isolated by construction, so one aborting says nothing about the others —
// only about the exit code the command owes its caller.
func (o Outcomes) Aborted() bool {
	for _, outcome := range o {
		if outcome.Aborted() {
			return true
		}
	}
	return false
}

// Recorded reports that at least one worktree gave an account of itself, which
// is what makes a recap worth printing.
func (o Outcomes) Recorded() bool {
	for _, outcome := range o {
		if outcome.Recorded() {
			return true
		}
	}
	return false
}

// One is the single outcome of a run over one worktree, and the zero value for
// any other arity. It is what the surfaces whose shape follows the arity read
// (LUC-198).
func (o Outcomes) One() Outcome {
	if len(o) != 1 {
		return Outcome{}
	}
	return o[0]
}

type RunParams struct {
	Service Service
	Sink    Sink
	Jobs    []domain.JobConfig
	// Profile is what the surface resolved Jobs from, carried through to the
	// Outcome so a recap can name it.
	Profile string
	WorkDir string
	// Worktree is WorkDir's branch, carried through to the Outcome and to every
	// Event so a merged report can name where a step happened.
	Worktree string
	LogDir   string
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
		worktree:   params.Worktree,
		logDir:     params.LogDir,
		env:        params.Env,
		prober:     params.Prober,
		project:    params.Project,
		proxyPort:  params.ProxyPort,
		nextConfig: params.NextConfig,
	}
	if r.nextConfig == nil {
		r.nextConfig = func(job domain.JobConfig) (string, string) {
			return detect.NextConfig(detect.NextConfigParams{WorkDir: params.WorkDir, Cwd: job.Cwd})
		}
	}
	if r.sink == nil {
		r.sink = noSink{}
	}
	return r.run(), ctx.Err()
}

type runner struct {
	ctx      context.Context
	service  Service
	sink     Sink
	jobs     []domain.JobConfig
	profile  string
	workDir  string
	worktree string
	logDir   string
	env      map[string]string
	prober   Prober
	// project and proxyPort together decide whether a job's URL is its name or
	// its port; both come from the surface, which is the side that reads config.
	project    string
	proxyPort  int
	nextConfig NextConfigLookup
	// servedPort is what the daemon answered its proxy is really on, and
	// noticedProxy records that the run has already explained a refusal — the
	// fact belongs to the run, not to each job that would repeat it.
	servedPort   int
	noticedProxy bool

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
		if result.PublicPort > 0 {
			r.servedPort = result.PublicPort
		}
		r.noticeProxyRefused(i + 1)

		alreadyRunning := result.Refused && job.Kind != domain.JobKindTask && rules.IsAlreadyRunning(result.Message)
		if result.Refused && !alreadyRunning {
			return r.abort(abortParams{Index: i, Job: job, Reason: result.Message, ExitCode: result.ExitCode})
		}

		if job.Kind == domain.JobKindTask {
			r.completed = append(r.completed, job.Name)
			r.results = append(r.results, domain.JobActionResult{Name: job.Name, Status: domain.JobActionDone, URL: r.jobURL(jobURLParams{Job: job, Ports: result.Ports, Host: host})})
			r.emit(Event{Phase: PhaseDone, Job: job.Name, Step: i + 1, Ports: result.Ports, URL: r.jobURL(jobURLParams{Job: job, Ports: result.Ports, Host: host})})
			continue
		}

		r.started = append(r.started, job.Name)
		r.results = append(r.results, domain.JobActionResult{Name: job.Name, Status: domain.JobActionStarted, URL: r.jobURL(jobURLParams{Job: job, Ports: result.Ports, Host: host})})
		if rules.ShouldProbeJob(job.Kind, result.Ports) {
			r.probeTargets = append(r.probeTargets, probeTarget{job: job.Name, resolved: result.Ports})
		}
		r.emit(Event{Phase: PhaseStarted, Job: job.Name, Step: i + 1, AlreadyRunning: alreadyRunning, Ports: result.Ports, URL: r.jobURL(jobURLParams{Job: job, Ports: result.Ports, Host: host}), DevOrigins: r.devOrigins(job, host)})
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
	if r.nextConfig == nil || host == "" || r.servedPort == 0 {
		return nil
	}
	path, source := r.nextConfig(job)
	if !rules.NeedsDevOrigins(rules.NeedsDevOriginsParams{Job: job, ConfigSource: source}) {
		return nil
	}
	return []domain.DevOriginFix{{
		Job:    job.Name,
		Config: path,
		Line:   fmt.Sprintf(domain.DevOriginsFixFmt, job.Name, rules.DevOriginsPattern(r.servedPort), path),
	}}
}

type jobURLParams struct {
	Job   domain.JobConfig
	Ports map[string]int
	Host  string
}

// jobURL answers with the port the daemon says it is really serving, never the
// one this run asked for: a name nothing serves is worse than a port.
func (r *runner) jobURL(params jobURLParams) string {
	return rules.JobURL(rules.JobURLParams{
		Job:        params.Job,
		Ports:      params.Ports,
		Host:       params.Host,
		PublicPort: r.servedPort,
	})
}

// noticeProxyRefused explains, once, why the names this run promised are not
// being served. Emitting it per job would bury the one fact it carries.
func (r *runner) noticeProxyRefused(step int) {
	if r.noticedProxy || r.proxyPort == 0 || r.servedPort != 0 {
		return
	}
	r.noticedProxy = true
	r.emit(Event{Phase: PhaseNotice, Step: step, Notice: fmt.Sprintf(domain.ProxyUnavailableFmt, r.proxyPort)})
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
	event.WorkDir = r.workDir
	event.Worktree = r.worktree
	r.sink.Emit(event)
}

func (r *runner) outcome() Outcome {
	return Outcome{
		WorkDir:   r.workDir,
		Worktree:  r.worktree,
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
