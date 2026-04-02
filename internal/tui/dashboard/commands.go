package dashboard

import (
	"bytes"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/state"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

func loadWorktrees(projectDir string, cfg domain.Config) tea.Cmd {
	return func() tea.Msg {
		statuses, err := worktree.List(worktree.ListParams{
			ProjectDir: projectDir,
			Config:     cfg,
		})
		if err != nil {
			return worktreeListMsg{Err: err}
		}

		currentState, _ := state.Load()

		return worktreeListMsg{
			Statuses:     statuses,
			ActiveBranch: currentState.ActiveWorktree,
		}
	}
}

func focusWorktree(projectDir string, branch string, cfg domain.Config) tea.Cmd {
	return func() tea.Msg {
		var buf bytes.Buffer

		err := worktree.Focus(worktree.FocusParams{
			ProjectDir: projectDir,
			Branch:     branch,
			Config:     cfg,
			Output:     &buf,
		})

		return focusDoneMsg{
			Branch: branch,
			Err:    err,
			Output: buf.String(),
		}
	}
}

func loadDetail(params worktree.DetailParams) tea.Cmd {
	return func() tea.Msg {
		return detailLoadedMsg{Detail: worktree.Detail(params)}
	}
}

