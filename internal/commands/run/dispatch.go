package run

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	downflow "github.com/LucasPcq/wtm/internal/flow/run/down"
	logsflow "github.com/LucasPcq/wtm/internal/flow/run/logs"
	startflow "github.com/LucasPcq/wtm/internal/flow/run/start"
	stopflow "github.com/LucasPcq/wtm/internal/flow/run/stop"
	upflow "github.com/LucasPcq/wtm/internal/flow/run/up"
)

// dispatchParams is an action `run list`'s picker chose, run in this process. It
// used to re-exec the binary for it (os.Executable + exec), which is what having
// no flow layer forced it into: the action lived in another command's runner and
// there was no other way to reach it.
type dispatchParams struct {
	Cmd *cobra.Command
	// WorkDir is the worktree the action acts on, loaded rather than assumed: the
	// config comes from it and not from the current directory.
	WorkDir string
	Job     string
	Profile string
	Format  string
}

// dispatchTarget is the configuration of the repository the picked job belongs to.
type dispatchTarget struct {
	config shared.ConfigResult
	run    domain.RunConfig
}

func (p dispatchParams) target() (dispatchTarget, error) {
	result, err := shared.LoadConfig(p.Cmd, p.WorkDir)
	if err != nil {
		return dispatchTarget{}, err
	}
	runCfg, err := config.LoadRun(result.StateDir)
	if err != nil {
		return dispatchTarget{}, fmt.Errorf("load run config: %w", err)
	}
	return dispatchTarget{config: result, run: runCfg}, nil
}

// prompter answers nothing: the picker already chose, so re-asking would let the
// reader undo the action they just picked.
func (p dispatchParams) prompter() shared.FlowPrompterParams {
	return shared.FlowPrompterParams{Interactive: false, Stderr: true}
}

func (p dispatchParams) dispatchStop() error {
	t, err := p.target()
	if err != nil {
		return err
	}
	_, err = stopflow.Run(stopflow.Params{
		Context:   shared.FlowContext(t.config),
		Request:   stopflow.Request{Cwd: p.WorkDir, Job: p.Job, Config: t.run},
		Prompter:  shared.FlowPrompter(p.prompter()),
		Presenter: stopPresenter{CLIPresenter: shared.NewPresenter(p.Cmd, p.Format)},
	})
	return err
}

func (p dispatchParams) dispatchStart() error {
	t, err := p.target()
	if err != nil {
		return err
	}
	outcome, err := startflow.Run(startflow.Params{
		Context:   shared.FlowContext(t.config),
		Request:   startflow.Request{Cwd: p.WorkDir, Job: p.Job, Config: t.run},
		Prompter:  shared.FlowPrompter(p.prompter()),
		Presenter: startPresenter{CLIPresenter: shared.NewPresenter(p.Cmd, p.Format)},
	})
	if err != nil {
		return err
	}
	if outcome.Aborted {
		return domain.ErrAborted
	}
	return nil
}

func (p dispatchParams) dispatchLogs() error {
	t, err := p.target()
	if err != nil {
		return err
	}
	_, err = logsflow.Run(logsflow.Params{
		Context:   shared.FlowContext(t.config),
		Request:   logsflow.Request{Cwd: p.WorkDir, Job: p.Job, Config: t.run},
		Prompter:  shared.FlowPrompter(p.prompter()),
		Presenter: logsPresenter{CLIPresenter: shared.NewPresenter(p.Cmd, p.Format)},
	})
	return err
}

func (p dispatchParams) dispatchUp() error {
	t, err := p.target()
	if err != nil {
		return err
	}
	outcome, err := upflow.Run(upflow.Params{
		Context: shared.FlowContext(t.config),
		Request: upflow.Request{Cwd: p.WorkDir, Profile: p.Profile, Config: t.run},
		// The picker asked its question already; the concurrency one it did not,
		// so it resolves to leaving the other worktrees alone.
		Prompter:  shared.FlowPrompter(p.prompter()),
		Presenter: upPresenter{CLIPresenter: shared.NewPresenter(p.Cmd, p.Format)},
	})
	if err != nil {
		return err
	}
	return concluded(outcome)
}

func (p dispatchParams) dispatchDown(all bool) error {
	t, err := p.target()
	if err != nil {
		return err
	}
	outcome, err := downflow.Run(downflow.Params{
		Context:   shared.FlowContext(t.config),
		Request:   downflow.Request{Cwd: p.WorkDir, Profile: p.Profile, All: all, Config: t.run},
		Prompter:  shared.FlowPrompter(p.prompter()),
		Presenter: downPresenter{CLIPresenter: shared.NewPresenter(p.Cmd, p.Format)},
	})
	if err != nil {
		return err
	}
	if outcome.Failed() {
		return domain.ErrAborted
	}
	return nil
}
