package dashboard

import (
	"io"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	ghservice "github.com/LucasPcq/wtm/internal/service/github"
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
	return execWtmCommand(func(err error) tea.Msg { return actionDoneMsg{Err: err} }, "wt", "create")
}

func execCleanWorktree(branch string) tea.Cmd {
	return execWtmCommand(func(err error) tea.Msg { return actionDoneMsg{Err: err} }, "wt", "clean", branch)
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

	cmd := exec.Command(bin, "svc", "up")
	cmd.Dir = projectDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return servicesStartedMsg{Err: err}
	})
}

func viewLogsCmd(worktreePath string) tea.Cmd {
	bin, err := os.Executable()
	if err != nil {
		return func() tea.Msg { return actionDoneMsg{Err: err} }
	}

	cmd := exec.Command(bin, "svc", "logs")
	cmd.Dir = worktreePath
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return actionDoneMsg{Err: err}
	})
}

func stopServicesCmd(worktreePath string, services []process.ServiceInfo) tea.Cmd {
	return func() tea.Msg {
		socketPath := process.SocketPath()
		if !process.IsDaemonRunning(socketPath) {
			return servicesStartedMsg{}
		}
		client := process.NewClient(socketPath)
		for _, svc := range services {
			if svc.WorkDir != worktreePath {
				continue
			}
			client.Send(process.Request{
				Action:  process.ActionStop,
				Name:    svc.Name,
				WorkDir: worktreePath,
			})
		}
		return servicesStartedMsg{}
	}
}

func execCreatePR(worktreePath string) tea.Cmd {
	bin, err := os.Executable()
	if err != nil {
		return func() tea.Msg { return actionDoneMsg{Err: err} }
	}

	cmd := exec.Command(bin, "pr", "create")
	cmd.Dir = worktreePath
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return actionDoneMsg{Err: err}
	})
}

func hasServicesConfig(projectDir string) bool {
	cfg, err := config.LoadServices(projectDir)
	if err != nil {
		return false
	}
	return len(cfg.Services) > 0
}

func loadPRs(projectDir string, filter domain.PRFilter) tea.Cmd {
	return func() tea.Msg {
		auth, err := config.LoadAuth()
		if err != nil {
			return prListMsg{Err: err}
		}

		prs, err := ghservice.ListPRs(ghservice.ListPRsParams{
			ProjectDir: projectDir,
			Filter:     filter,
			Username:   auth.User,
		})
		if err != nil {
			return prListMsg{Err: err}
		}

		return prListMsg{PRs: prs}
	}
}

func loadPRDetail(projectDir string, number int) tea.Cmd {
	return func() tea.Msg {
		pr, err := ghservice.GetPRDetail(ghservice.GetPRDetailParams{
			ProjectDir: projectDir,
			Number:     number,
		})
		if err != nil {
			return prDetailMsg{Err: err}
		}
		return prDetailMsg{PR: pr}
	}
}

func openInBrowser(url string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("open", url)
		cmd.Run()
		return logMsg("Opened in browser")
	}
}
