// Package up runs the `wtm run up` flow.
package up

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/seam"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
)

type Request struct {
	// Worktrees are the positionals as they were typed; empty leaves the step to
	// ask, or to answer with the current worktree.
	Worktrees []string
	// Cwd is where the command was launched, the worktree step's safe default.
	Cwd     string
	Profile string
	// Exclusive and Parallel override the project's standing preference for one
	// run. They are the concurrency step's Resolve, not a second axis.
	Exclusive bool
	Parallel  bool
	NoProbe   bool
	// Config is run.toml, already validated by the surface that read it.
	Config domain.RunConfig
}

type Outcome struct {
	// WorkDirs are the worktrees this run acted on, in selection order.
	WorkDirs []string
	Profile  string
	// Results is one account per worktree. A run over one is a slice of one:
	// the surfaces read the arity rather than branching on a mode (LUC-198).
	Results runlogs.Outcomes
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

// Operation declares how a surface must schedule a run up: it gives the surface
// back and holds the worktree it started jobs in, named by the worktree step.
func Operation() flow.Operation {
	return flow.Operation{
		Kind:      domain.OpKindRunUp,
		Mode:      flow.ModeBackground,
		TargetKey: target.KeyWorktree,
	}
}

func Run(params Params) (Outcome, error) {
	f := &upFlow{
		ctx:       params.Context,
		request:   params.Request,
		prompter:  params.Prompter,
		presenter: params.Presenter,
	}
	return f.run()
}

type upFlow struct {
	ctx       flow.Context
	request   Request
	prompter  flow.Prompter
	presenter Presenter

	// named are the worktrees the positionals designated, nil when there were none.
	named []target.Resolved
	// jobs and running are one reading of the daemon's index: what runs where,
	// for the worktree badges and for the concurrency question.
	jobs    []domain.JobInfo
	running map[string]int
	service runlogs.Service
}

func (f *upFlow) run() (Outcome, error) {
	named, err := target.NamedAll(target.ResolveAllParams{ProjectDir: f.ctx.ProjectDir, Queries: f.request.Worktrees})
	if err != nil {
		return Outcome{}, err
	}
	f.named = named

	// A flag that contradicts itself is not a decision to default: --exclusive
	// means one stack at a time, and it cannot be applied to a run that brings up
	// several. Refused here rather than after the wizard so a run that cannot
	// happen does not wake a daemon first; the picker refuses the same thing at
	// the tick, through the step's ValidateSet.
	if f.request.Exclusive && len(named) > 1 {
		return Outcome{}, domain.ErrExclusiveMultiWorktree
	}

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

	if err := f.remember(answers); err != nil {
		return Outcome{}, err
	}
	f.noticeOverridden(answers)
	if err := f.clearOthers(answers); err != nil {
		return Outcome{}, err
	}

	return f.start(answers)
}

// connect wakes the daemon and reads its index once. Both the worktree badges
// and the concurrency question are about what is already running, so neither
// can be built before this.
func (f *upFlow) connect() error {
	return f.presenter.Stage(flow.StageParams{
		Message: domain.RunDaemonConnecting,
		Work: func() error {
			if err := process.EnsureDaemon(process.DaemonParams{
				SocketPath: process.SocketPath(),
				ProxyPort:  rules.ProxyPort(f.ctx.Config.Global),
			}); err != nil {
				return fmt.Errorf("ensure daemon: %w", err)
			}
			f.service = runlogs.NewService(runlogs.ServiceParams{SocketPath: process.SocketPath()})
			// A daemon that cannot list is not a reason to refuse the run: the
			// counts decorate a picker and the question defaults to stopping
			// nothing.
			f.jobs, _ = f.service.List("")
			f.running = rules.RunningJobsByWorktree(f.jobs)
			return nil
		},
	})
}

// remember writes the answer to run.toml when the user asked for it to stand.
// It is never silent: a file changed without a word is a file nobody knows to
// change back.
func (f *upFlow) remember(answers flow.Answers) error {
	answer := answers.Value(KeyConcurrency)
	if !remembers(answer) {
		return nil
	}

	cfg := f.request.Config
	cfg.Concurrency = concurrencyOf(answer)
	if err := runconfig.Save(runconfig.SaveParams{StateDir: f.ctx.StateDir, Config: cfg}); err != nil {
		return fmt.Errorf("remember concurrency: %w", err)
	}
	f.request.Config = cfg
	f.presenter.Status(flow.Notice{
		Kind: flow.NoticeMessage,
		Text: fmt.Sprintf(domain.RunConcurrencyRememberedFmt, cfg.Concurrency),
	})
	return nil
}

// noticeOverridden says the project's settled answer could not be applied to
// this run. It is only ever reached where nobody could be asked: the safe
// default destroys nothing, and a default that goes unsaid is a default nobody
// can correct.
func (f *upFlow) noticeOverridden(answers flow.Answers) {
	if answers.Answered(KeyConcurrency) || !f.decideConcurrency(answers).Contradiction {
		return
	}
	f.presenter.Status(warning(fmt.Sprintf(domain.RunConcurrencyOverriddenFmt,
		f.request.Config.Concurrency, len(f.workDirs(answers)))))
}

// clearOthers stops the other worktrees' jobs when that is what was decided.
// A worktree that refuses to stop is reported and the run carries on: the
// answer was about this machine's load, not about a dependency.
func (f *upFlow) clearOthers(answers flow.Answers) error {
	if f.concurrency(answers) != domain.ConcurrencyExclusive {
		return nil
	}
	others := f.otherWorktrees(answers)
	if len(others) == 0 {
		return nil
	}

	dirs := make([]string, 0, len(others))
	for dir := range others {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	client := process.NewClient(process.SocketPath())
	return f.presenter.Stage(flow.StageParams{
		Message: domain.RunStoppingOthers,
		Work: func() error {
			for _, dir := range dirs {
				resp, err := client.Send(process.Request{Action: process.ActionStopAll, WorkDir: dir})
				switch {
				case err != nil:
					f.presenter.Status(warning(fmt.Sprintf(domain.RunStopOtherFailFmt, filepath.Base(dir), err)))
				case resp.Status == process.StatusError:
					f.presenter.Status(warning(fmt.Sprintf(domain.RunStopOtherFailFmt, filepath.Base(dir), resp.Message)))
				default:
					f.presenter.Status(flow.Notice{
						Kind: flow.NoticeSuccess,
						Text: fmt.Sprintf(domain.RunStoppedOtherFmt, filepath.Base(dir)),
					})
				}
			}
			return nil
		},
	})
}

func warning(text string) flow.Notice {
	return flow.Notice{Kind: flow.NoticeWarning, Text: text}
}

func (f *upFlow) start(answers flow.Answers) (Outcome, error) {
	workDirs := f.workDirs(answers)
	profile, err := f.resolveProfile(answers)
	if err != nil {
		return Outcome{}, err
	}

	set := seam.OpenSet(seam.SetParams{
		ProjectDir:  f.ctx.ProjectDir,
		StateDir:    f.ctx.StateDir,
		WorkDirs:    workDirs,
		Jobs:        profile.Jobs,
		ProbeBudget: rules.PortProbeBudget(f.request.Config),
		NoProbe:     f.request.NoProbe,
		ProxyPort:   rules.ProxyPort(f.ctx.Config.Global),
	})

	results, err := f.presenter.Sequence(seam.SequenceParams{
		Board:     set.Board(),
		Profile:   profile.Name,
		Worktrees: set.Worktrees(),
		Start:     set.Starter(seam.StartParams{Profile: profile.Name, Jobs: profile.Jobs}),
	})
	if err != nil {
		return Outcome{}, err
	}
	return Outcome{
		WorkDirs: workDirs,
		Profile:  profile.Name,
		Results:  results,
		Aborted:  results.Aborted(),
	}, nil
}

// resolvedProfile is what this run settled on: a name for it and the jobs it
// starts. The name is empty for a config declaring no profile at all — dropping
// it left `run up` unable to say which of several it had brought up (LUC-208).
type resolvedProfile struct {
	Name string
	Jobs []domain.JobConfig
}

func (f *upFlow) resolveProfile(answers flow.Answers) (resolvedProfile, error) {
	name := answers.Value(target.KeyProfile)
	if name == "" {
		profile, ok := rules.DefaultProfile(f.request.Config)
		if !ok {
			return resolvedProfile{Jobs: rules.JobsWithoutProfile(f.request.Config)}, nil
		}
		return f.profileRun(profile), nil
	}

	profile, ok := rules.FindProfile(f.request.Config, name)
	if !ok {
		return resolvedProfile{}, fmt.Errorf("profile %q not found in config", name)
	}
	return f.profileRun(profile), nil
}

func (f *upFlow) profileRun(profile domain.ProfileConfig) resolvedProfile {
	return resolvedProfile{Name: profile.Name, Jobs: rules.ProfileJobs(f.request.Config, profile)}
}
