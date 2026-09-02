// Package seam binds a run flow to the daemon holding a worktree's jobs: the
// board a surface lists, the environment those jobs are given, and the start
// sequence a surface drives. It assumes the daemon is up — opening one is the
// caller's business, because only it knows the proxy port.
package seam

import (
	"context"
	"path/filepath"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/portprobe"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

type Params struct {
	ProjectDir string
	StateDir   string
	// WorkDir is the worktree this seam is about, as git spells it.
	WorkDir string
	// Jobs are what the board lists. `run up` passes the profile it resolved, so
	// the view shows the run rather than every job run.toml declares beside it,
	// with the previous run's log behind each (LUC-208); `run logs` passes them
	// all, which is what it is for.
	Jobs []domain.JobConfig
	// ProxyPort is where the run proxy serves the jobs' names. Zero leaves them
	// on their own ports.
	ProxyPort int
	// ProbeBudget is how long the port check may dial for; zero or NoProbe skips
	// it entirely.
	ProbeBudget time.Duration
	NoProbe     bool
}

type Seam struct {
	service   runlogs.Service
	board     runlogs.Board
	workDir   string
	logDir    string
	env       map[string]string
	prober    runlogs.Prober
	project   string
	proxyPort int
}

func Open(params Params) Seam {
	logDir := LogDir(LogDirParams{StateDir: params.StateDir, WorkDir: params.WorkDir})
	service := runlogs.NewService(runlogs.ServiceParams{SocketPath: process.SocketPath()})
	return Seam{
		service: service,
		board: runlogs.NewBoard(runlogs.BoardParams{
			Service: service,
			Jobs:    params.Jobs,
			WorkDir: params.WorkDir,
			LogDir:  logDir,
		}),
		workDir: params.WorkDir,
		logDir:  logDir,
		env: JobEnv(JobEnvParams{
			ProjectDir: params.ProjectDir,
			StateDir:   params.StateDir,
			WorkDir:    params.WorkDir,
		}),
		prober:    newProber(params.ProbeBudget, params.NoProbe),
		project:   filepath.Base(params.ProjectDir),
		proxyPort: params.ProxyPort,
	}
}

func (s Seam) Board() runlogs.Board     { return s.board }
func (s Seam) Service() runlogs.Service { return s.service }
func (s Seam) Env() map[string]string   { return s.env }
func (s Seam) LogDir() string           { return s.logDir }
func (s Seam) Project() string          { return s.project }
func (s Seam) ProxyPort() int           { return s.proxyPort }

type StartParams struct {
	Profile string
	Jobs    []domain.JobConfig
}

// Starter is the start sequence as a surface drives it: it draws first, then
// calls what this returns.
func (s Seam) Starter(params StartParams) runlogs.StartFunc {
	return func(ctx context.Context, sink runlogs.Sink) (runlogs.Outcome, error) {
		return runlogs.Run(ctx, runlogs.RunParams{
			Service:   s.service,
			Sink:      sink,
			Jobs:      params.Jobs,
			Profile:   params.Profile,
			WorkDir:   s.workDir,
			LogDir:    s.logDir,
			Env:       s.env,
			Prober:    s.prober,
			Project:   s.project,
			ProxyPort: s.proxyPort,
		})
	}
}

type LogDirParams struct {
	StateDir string
	WorkDir  string
}

// LogDir resolves where the daemon persists this worktree's job logs. The
// branch is looked up here rather than passed along by the daemon, which must
// never run git; a worktree with no branch, or one git cannot name, persists
// nothing rather than sharing another's directory.
func LogDir(params LogDirParams) string {
	branch, err := worktree.CurrentBranch(worktree.CurrentBranchParams{Dir: params.WorkDir})
	if err != nil {
		return ""
	}
	return rules.WorktreeLogDir(rules.WorktreeLogDirParams{StateDir: params.StateDir, Branch: branch})
}

type JobEnvParams struct {
	ProjectDir string
	StateDir   string
	WorkDir    string
}

// JobEnv resolves the worktree-scoped environment handed to every job of this
// run. Like LogDir it degrades to nothing rather than to another worktree's
// values: a run whose worktree cannot be named injects no isolation instead of
// the wrong one.
func JobEnv(params JobEnvParams) map[string]string {
	env, err := worktree.JobEnv(worktree.JobEnvParams{
		ProjectDir: params.ProjectDir,
		StateDir:   params.StateDir,
		Dir:        params.WorkDir,
	})
	if err != nil {
		return nil
	}
	return env
}

// dialProber is this side of the runlogs.Prober seam: the run says which ports
// to check, here owns the budget and the socket.
type dialProber struct{ budget time.Duration }

func (p dialProber) Listening(ctx context.Context, ports []int, settled func(map[int]bool) bool) map[int]bool {
	return portprobe.Poll(ctx, portprobe.PollParams{Ports: ports, Budget: p.budget, Settled: settled})
}

// newProber returns nil when the check is switched off, which is what the run
// reads to skip it entirely.
func newProber(budget time.Duration, disabled bool) runlogs.Prober {
	if disabled || budget <= 0 {
		return nil
	}
	return dialProber{budget: budget}
}

// SequenceParams is the hand-over from a run to the surface watching it: what
// the surface lists, what it calls when it is ready to report, and what to call
// the run in a header.
type SequenceParams struct {
	Board runlogs.Board
	// Profile and Job name the run, exactly one of them set: a profile for
	// `run up`, a job for `run start`.
	Profile string
	Job     string
	Start   runlogs.StartFunc
	// Inline says the run blocks until it ends — a task, whose output belongs to
	// the scrollback rather than to a screen given back when it exits. It is the
	// flow that knows, because it is the flow that resolved the job.
	Inline bool
}

// Watcher is the half of a run's Presenter that shows the start. It is the one
// thing a run cannot report through Stage: the surface has to be drawing before
// the first job is asked for, so it is the surface that calls Start.
type Watcher interface {
	Sequence(SequenceParams) (runlogs.Outcome, error)
}
