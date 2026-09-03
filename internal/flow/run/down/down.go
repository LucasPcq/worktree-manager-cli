// Package down runs the `wtm run down` flow.
package down

import (
	"errors"
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
)

type Request struct {
	Worktree string
	Cwd      string
	// Profile narrows the stop to one profile's jobs. Empty means everything the
	// worktree has up, which is the command's safe default — so it is never asked.
	Profile string
	// All reaches across every worktree, which is why it takes no worktree and no
	// profile: it is a different question, not a wider answer to this one.
	All    bool
	Config domain.RunConfig
}

type Outcome struct {
	WorkDir string
	Profile string
	All     bool
	Results []domain.JobActionResult
	// NoDaemon says nothing was listening, so nothing was running to stop.
	NoDaemon bool
	Aborted  bool
}

// Failed reports a job left standing. Every run command exits non-zero on what
// it could not do (LUC-198).
func (o Outcome) Failed() bool {
	for _, result := range o.Results {
		if result.Status == domain.JobActionError {
			return true
		}
	}
	return false
}

type Presenter interface {
	flow.Presenter
	Downed(Outcome) error
}

type Params struct {
	Context   flow.Context
	Request   Request
	Prompter  flow.Prompter
	Presenter Presenter
}

// Operation declares how a surface schedules a down: it holds the worktree it
// is emptying, and gives the surface back.
func Operation() flow.Operation {
	return flow.Operation{
		Kind:      domain.OpKindRunDown,
		Mode:      flow.ModeBackground,
		TargetKey: target.KeyWorktree,
	}
}

func Run(params Params) (Outcome, error) {
	f := &downFlow{
		ctx:       params.Context,
		request:   params.Request,
		prompter:  params.Prompter,
		presenter: params.Presenter,
	}
	return f.run()
}

type downFlow struct {
	ctx       flow.Context
	request   Request
	prompter  flow.Prompter
	presenter Presenter

	named *target.Resolved
}

func (f *downFlow) run() (Outcome, error) {
	named, err := target.Named(target.ResolveParams{ProjectDir: f.ctx.ProjectDir, Query: f.request.Worktree})
	if err != nil {
		return Outcome{}, err
	}
	f.named = named

	answers, err := f.prompter.Ask(f.session())
	if errors.Is(err, domain.ErrUserAborted) {
		f.presenter.Notice(flow.AbortedNotice)
		return Outcome{Aborted: true}, nil
	}
	if err != nil {
		return Outcome{}, err
	}

	outcome := Outcome{
		Profile: f.request.Profile,
		All:     f.request.All,
	}
	if !f.request.All {
		outcome.WorkDir = target.WorkDir(target.WorkDirParams{Answers: answers, Named: f.named, Cwd: f.request.Cwd})
	}

	if err := f.wake(outcome.WorkDir); err != nil {
		return Outcome{}, err
	}
	if !process.IsDaemonRunning(process.SocketPath()) {
		outcome.NoDaemon = true
		return outcome, f.presenter.Downed(outcome)
	}

	results, err := f.stop(outcome)
	if err != nil {
		return Outcome{}, err
	}
	outcome.Results = results
	return outcome, f.presenter.Downed(outcome)
}

// wake starts a daemon when the index still holds jobs for what is being
// stopped. A daemon exits once no foreground job is left, so after a reboot —
// or simply half an hour later — nothing is listening while detached stacks are
// very much up, and `run down` is exactly the command that must reach them.
func (f *downFlow) wake(workDir string) error {
	if process.IsDaemonRunning(process.SocketPath()) {
		return nil
	}
	indexed := process.HasIndexedJobs(workDir)
	if f.request.All {
		indexed = process.HasAnyIndexedJob()
	}
	if !indexed {
		return nil
	}
	return f.presenter.Stage(flow.StageParams{
		Message: domain.RunDaemonConnecting,
		Work: func() error {
			return process.EnsureDaemon(process.DaemonParams{
				SocketPath: process.SocketPath(),
				ProxyPort:  rules.ProxyPort(f.ctx.Config.Global),
			})
		},
	})
}

func (f *downFlow) stop(outcome Outcome) ([]domain.JobActionResult, error) {
	if outcome.Profile != "" {
		return f.stopProfile(outcome)
	}
	return f.stopAll(outcome)
}

// stopProfile stops the profile's jobs one by one, so a job that refuses is
// named rather than lost inside a single failure for the whole set.
func (f *downFlow) stopProfile(outcome Outcome) ([]domain.JobActionResult, error) {
	profile, ok := rules.FindProfile(f.request.Config, outcome.Profile)
	if !ok {
		return nil, fmt.Errorf("profile %q not found in config", outcome.Profile)
	}

	client := process.NewClient(process.SocketPath())
	jobs := rules.ProfileJobs(f.request.Config, profile)
	results := make([]domain.JobActionResult, 0, len(jobs))
	for _, job := range jobs {
		var resp process.Response
		err := f.presenter.Stage(flow.StageParams{
			Message: fmt.Sprintf(domain.RunStoppingFmt, job.Name),
			Work: func() error {
				var sendErr error
				resp, sendErr = client.Send(process.Request{
					Action:  process.ActionStop,
					Name:    job.Name,
					WorkDir: outcome.WorkDir,
				})
				return sendErr
			},
		})
		switch {
		case err != nil:
			results = append(results, domain.JobActionResult{Name: job.Name, Status: domain.JobActionError, Message: err.Error()})
		case resp.Status == process.StatusError:
			results = append(results, domain.JobActionResult{Name: job.Name, Status: domain.JobActionError, Message: resp.Message})
		default:
			results = append(results, domain.JobActionResult{Name: job.Name, Status: domain.JobActionStopped})
		}
	}
	return results, nil
}

func (f *downFlow) stopAll(outcome Outcome) ([]domain.JobActionResult, error) {
	request := process.Request{Action: process.ActionStopAll}
	if !outcome.All {
		request.WorkDir = outcome.WorkDir
	}

	var resp process.Response
	if err := f.presenter.Stage(flow.StageParams{
		Message: domain.RunStoppingJobs,
		Work: func() error {
			var sendErr error
			resp, sendErr = client().Send(request)
			return sendErr
		},
	}); err != nil {
		return nil, fmt.Errorf("stop all jobs: %w", err)
	}
	if resp.Status == process.StatusError {
		return nil, fmt.Errorf("stop all: %s", resp.Message)
	}

	stopped := make([]domain.JobActionResult, 0, len(resp.Jobs))
	for _, job := range resp.Jobs {
		stopped = append(stopped, domain.JobActionResult{Name: job.Name, Status: domain.JobActionStopped})
	}
	return stopped, nil
}

func client() *process.Client { return process.NewClient(process.SocketPath()) }

// session asks for the worktree and nothing else. `run down` with no --profile
// means "stop everything here", which is a safe default: asking would force a
// choice the command does not need and offers no "all" answer for. With --all
// there is no worktree to ask about either.
func (f *downFlow) session() flow.Session {
	if f.request.All {
		return flow.Session{ErrLabel: domain.CmdDown}
	}
	return flow.Session{
		ErrLabel: domain.CmdDown,
		Presets:  target.Presets(target.PresetParams{Named: f.named, Profile: f.request.Profile}),
		Steps: []flow.Step{
			target.WorktreeStep(target.WorktreeParams{
				ProjectDir: f.ctx.ProjectDir,
				Current:    f.request.Cwd,
				Running:    f.running(),
			}),
		},
	}
}

func (f *downFlow) running() map[string]int {
	socket := process.SocketPath()
	if !process.IsDaemonRunning(socket) {
		return nil
	}
	return target.RunningJobs(runlogs.NewService(runlogs.ServiceParams{SocketPath: socket}))
}
