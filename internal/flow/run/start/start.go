// Package start runs the `wtm run start` flow.
package start

import (
	"errors"
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/seam"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
)

type Request struct {
	// Worktree is the positional as it was typed; Cwd is what answers for it
	// when nobody is asked.
	Worktree string
	Cwd      string
	Job      string
	// Config is run.toml, so a job name matching nothing fails here rather than
	// at the daemon.
	Config domain.RunConfig
}

type Outcome struct {
	WorkDir string
	Job     domain.JobConfig
	Result  runlogs.Outcome
	Aborted bool
}

type Presenter interface {
	flow.Presenter
	seam.Watcher
}

type Params struct {
	Context   flow.Context
	Request   Request
	Prompter  flow.Prompter
	Presenter Presenter
}

// Operation declares how a surface schedules a single start: like `run up` it
// gives the surface back and holds the worktree it started the job in.
func Operation() flow.Operation {
	return flow.Operation{
		Kind:      domain.OpKindRunStart,
		Mode:      flow.ModeBackground,
		TargetKey: target.KeyWorktree,
	}
}

func Run(params Params) (Outcome, error) {
	f := &startFlow{
		ctx:       params.Context,
		request:   params.Request,
		prompter:  params.Prompter,
		presenter: params.Presenter,
	}
	return f.run()
}

type startFlow struct {
	ctx       flow.Context
	request   Request
	prompter  flow.Prompter
	presenter Presenter

	named   *target.Resolved
	running map[string]int
}

func (f *startFlow) run() (Outcome, error) {
	named, err := target.Named(target.ResolveParams{ProjectDir: f.ctx.ProjectDir, Query: f.request.Worktree})
	if err != nil {
		return Outcome{}, err
	}
	f.named = named

	if err := f.connect(); err != nil {
		return Outcome{}, err
	}

	answers, err := f.prompter.Ask(f.session())
	if errors.Is(err, domain.ErrUserAborted) {
		f.presenter.Notice(flow.AbortedNotice)
		return Outcome{Aborted: true}, nil
	}
	if err != nil {
		return Outcome{}, err
	}

	job, err := target.DeclaredJob(f.request.Config, answers.Value(target.KeyJob))
	if err != nil {
		return Outcome{}, err
	}

	workDir := target.WorkDir(target.WorkDirParams{
		Answers: answers,
		Named:   f.named,
		Cwd:     f.request.Cwd,
	})
	runSeam := seam.Open(seam.Params{
		ProjectDir: f.ctx.ProjectDir,
		StateDir:   f.ctx.StateDir,
		WorkDir:    workDir,
		// The board lists every declared job, not just this one: starting a job is
		// no reason to hide the ones already up beside it.
		Jobs:      f.request.Config.Jobs,
		ProxyPort: rules.ProxyPort(f.ctx.Config.Global),
	})

	result, err := f.presenter.Sequence(seam.SequenceParams{
		Board:  runSeam.Board(),
		Job:    job.Name,
		Inline: job.Kind == domain.JobKindTask,
		Start:  runSeam.Starter(seam.StartParams{Jobs: []domain.JobConfig{job}}),
	})
	if err != nil {
		return Outcome{}, err
	}
	return Outcome{WorkDir: workDir, Job: job, Result: result, Aborted: result.Aborted()}, nil
}

// connect wakes the daemon before anything is asked: the worktree picker shows
// what each worktree is already running, which only the daemon knows.
func (f *startFlow) connect() error {
	return f.presenter.Stage(flow.StageParams{
		Message: domain.RunDaemonConnecting,
		Work: func() error {
			if err := process.EnsureDaemon(process.DaemonParams{
				SocketPath: process.SocketPath(),
				ProxyPort:  rules.ProxyPort(f.ctx.Config.Global),
			}); err != nil {
				return fmt.Errorf("ensure daemon: %w", err)
			}
			f.running = target.RunningJobs(runlogs.NewService(runlogs.ServiceParams{SocketPath: process.SocketPath()}))
			return nil
		},
	})
}

func (f *startFlow) session() flow.Session {
	return flow.Session{
		ErrLabel: domain.CmdStart,
		Presets:  target.Presets(target.PresetParams{Named: f.named, Job: f.request.Job}),
		Steps: []flow.Step{
			target.WorktreeStep(target.WorktreeParams{
				ProjectDir: f.ctx.ProjectDir,
				Current:    f.request.Cwd,
				Running:    f.running,
			}),
			target.JobStep(target.JobParams{Jobs: f.request.Config.Jobs, Flag: domain.FlagJob}),
		},
	}
}
