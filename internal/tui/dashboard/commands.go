package dashboard

import (
	"bytes"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/state"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

// tuiWriter implements io.Writer and sends each write as a tea.Msg to the program.
type tuiWriter struct {
	program *tea.Program
}

func (w *tuiWriter) Write(p []byte) (int, error) {
	w.program.Send(hookOutputMsg{Line: string(p)})
	return len(p), nil
}

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

func focusWorktree(projectDir string, branch string, cfg domain.Config, program *tea.Program) tea.Cmd {
	return func() tea.Msg {
		var output *tuiWriter
		if program != nil {
			output = &tuiWriter{program: program}
		}

		var buf bytes.Buffer
		writer := output
		if writer == nil {
			// Fallback: capture in buffer for error reporting
			return focusDoneMsg{
				Branch: branch,
				Err:    worktree.Focus(worktree.FocusParams{ProjectDir: projectDir, Branch: branch, Config: cfg, Output: &buf}),
				Output: buf.String(),
			}
		}

		err := worktree.Focus(worktree.FocusParams{
			ProjectDir: projectDir,
			Branch:     branch,
			Config:     cfg,
			Output:     writer,
		})

		return focusDoneMsg{
			Branch: branch,
			Err:    err,
		}
	}
}

func loadDetail(params worktree.DetailParams) tea.Cmd {
	return func() tea.Msg {
		return detailLoadedMsg{Detail: worktree.Detail(params)}
	}
}

func execNewWorktree() tea.Cmd {
	bin, err := os.Executable()
	if err != nil {
		return func() tea.Msg { return actionDoneMsg{Err: err} }
	}

	cmd := exec.Command(bin, "new")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return actionDoneMsg{Err: err}
	})
}

func execCleanWorktree(branch string) tea.Cmd {
	bin, err := os.Executable()
	if err != nil {
		return func() tea.Msg { return actionDoneMsg{Err: err} }
	}

	cmd := exec.Command(bin, "clean", branch)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return actionDoneMsg{Err: err}
	})
}
