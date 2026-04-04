package dashboard

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
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
		var output io.Writer = io.Discard
		if program != nil {
			output = &tuiWriter{program: program}
		}

		err := worktree.Focus(worktree.FocusParams{
			ProjectDir: projectDir, Branch: branch, Config: cfg, Output: output,
		})

		return focusDoneMsg{Branch: branch, Err: err}
	}
}

func loadDetail(params worktree.DetailParams) tea.Cmd {
	return func() tea.Msg {
		return detailLoadedMsg{Detail: worktree.Detail(params)}
	}
}

func execWtmCommand(resultMsg func(error) tea.Msg, args ...string) tea.Cmd {
	bin, err := os.Executable()
	if err != nil {
		return func() tea.Msg { return resultMsg(err) }
	}

	cmd := exec.Command(bin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return tea.ExecProcess(cmd, resultMsg)
}

func execNewWorktree() tea.Cmd {
	return execWtmCommand(func(err error) tea.Msg { return actionDoneMsg{Err: err} }, "new")
}

func execCleanWorktree(branch string) tea.Cmd {
	return execWtmCommand(func(err error) tea.Msg { return actionDoneMsg{Err: err} }, "clean", branch)
}

func loadServiceStatuses() tea.Cmd {
	return func() tea.Msg {
		socketPath := process.SocketPath()
		if !process.IsDaemonRunning(socketPath) {
			return serviceListMsg{}
		}
		client := process.NewClient(socketPath)
		resp, err := client.Send(process.Request{Action: process.ActionList})
		if err != nil {
			return serviceListMsg{Err: err}
		}
		return serviceListMsg{Services: resp.Services}
	}
}

func startServicesCmd(projectDir string) tea.Cmd {
	bin, err := os.Executable()
	if err != nil {
		return func() tea.Msg { return servicesStartedMsg{Err: err} }
	}

	cmd := exec.Command(bin, "up")
	cmd.Dir = projectDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return servicesStartedMsg{Err: err}
	})
}

func attachServiceCmd(serviceName string) tea.Cmd {
	return execWtmCommand(func(err error) tea.Msg { return actionDoneMsg{Err: err} }, "logs", serviceName)
}

func stopServicesCmd() tea.Cmd {
	return func() tea.Msg {
		socketPath := process.SocketPath()
		if !process.IsDaemonRunning(socketPath) {
			return servicesStartedMsg{}
		}
		client := process.NewClient(socketPath)
		resp, err := client.Send(process.Request{Action: process.ActionStopAll})
		if err != nil {
			return servicesStartedMsg{Err: err}
		}
		if resp.Status != process.StatusOK {
			return servicesStartedMsg{Err: fmt.Errorf("%s", resp.Message)}
		}
		return servicesStartedMsg{}
	}
}

func hasServicesConfig(projectDir string) bool {
	cfg, err := config.LoadServices(projectDir)
	if err != nil {
		return false
	}
	return len(cfg.Services) > 0
}
