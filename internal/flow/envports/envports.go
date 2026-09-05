// Package envports settles the host ports a freshly provisioned .env holds onto
// the ones the worktree it belongs to actually binds.
package envports

import (
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
	envsvc "github.com/LucasPcq/wtm/internal/service/env"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

type Params struct {
	Context      flow.Context
	Branch       string
	WorktreePath string
	Prompter     flow.Prompter
	Presenter    flow.Presenter
}

// Settle moves the host ports a freshly provisioned .env holds onto the ones
// this worktree binds. The values were just copied from main or from a parent,
// so they carry that worktree's ports and nothing else would fix them.
//
// It applies without asking when nobody can be asked: leaving a .env pointing at
// another worktree's services does not make the run safer, it makes it useless.
func Settle(params Params) error {
	resolved, err := worktree.ResolveEnvPorts(worktree.ResolveEnvPortsParams{
		ProjectDir:   params.Context.ProjectDir,
		StateDir:     params.Context.StateDir,
		Branch:       params.Branch,
		WorktreePath: params.WorktreePath,
		EnvFiles:     params.Context.Config.Project.Env.Files,
		Global:       params.Context.Config.Global,
	})
	if err != nil || resolved.Empty() {
		return err
	}

	plan, err := envsvc.ComputeEnvPorts(resolved)
	if err != nil {
		return err
	}
	if anomalies := rules.EnvPortAnomalyLines(plan); len(anomalies) > 0 {
		params.Presenter.Status(flow.Notice{Kind: flow.NoticeWarning, Text: domain.EnvPortAnomaliesTitle, Lines: anomalies})
	}
	for _, notice := range rules.EnvPortNotices(plan) {
		params.Presenter.Status(flow.Notice{Kind: flow.NoticeWarning, Text: notice.Title, Lines: []string{notice.Line}})
	}
	if owned := rules.OwnedEnvLines(plan); len(owned) > 0 {
		params.Presenter.Status(flow.Notice{Kind: flow.NoticeMessage, Text: domain.EnvOwnedKeysTitle, Lines: owned})
	}

	if len(rules.EnvPortRewrites(plan)) == 0 {
		return envsvc.ApplyOwnedEnv(resolved)
	}

	// Asked, the table lives inside the question; applied without asking, it is a
	// report of what just happened. Same lines, two different acts.
	if params.Prompter.Interactive() {
		proceed, confirmErr := params.Prompter.Confirm(flow.ConfirmParams{
			Title:       rules.EnvPortsConfirmTitle(plan),
			Description: rules.EnvPortPromptDescription(plan),
			DefaultYes:  true,
		})
		if confirmErr != nil || !proceed {
			return envsvc.ApplyOwnedEnv(resolved)
		}
	} else {
		params.Presenter.Status(flow.Notice{
			Kind:  flow.NoticeMessage,
			Text:  rules.EnvPortOffsetLabel(plan.Offset),
			Lines: rules.EnvPortTableLines(plan),
		})
	}

	_, err = envsvc.ApplyEnvPorts(resolved)
	return err
}
