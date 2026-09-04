package runlogs

import (
	"fmt"
	"sync"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type BoardParams struct {
	Service Service
	// Jobs are the worktree's declared jobs, in the order a surface lists them.
	Jobs    []domain.JobConfig
	WorkDir string
	// Worktree is WorkDir's branch, what a merged board shows above this
	// worktree's rows.
	Worktree string
	LogDir   string
}

// NewBoard builds the surface's view of a worktree's jobs. It reads nothing
// until Refresh: a surface opens before the daemon is asked anything, and a job
// nobody has started yet is still a job to show.
func NewBoard(params BoardParams) Board {
	return &board{
		service:  params.Service,
		jobs:     params.Jobs,
		workDir:  params.WorkDir,
		worktree: params.Worktree,
		logDir:   params.LogDir,
	}
}

type board struct {
	service  Service
	jobs     []domain.JobConfig
	workDir  string
	worktree string
	logDir   string

	// mu guards live, which a surface refreshes off the goroutine that renders it.
	mu        sync.RWMutex
	live      map[string]domain.JobInfo
	liveOrder []string
}

func (b *board) Refresh() error {
	infos, err := b.service.List(b.workDir)
	if err != nil {
		return err
	}

	live := make(map[string]domain.JobInfo, len(infos))
	order := make([]string, 0, len(infos))
	for _, info := range infos {
		if info.WorkDir != b.workDir {
			continue
		}
		if _, known := live[info.Name]; !known {
			order = append(order, info.Name)
		}
		live[info.Name] = info
	}

	b.mu.Lock()
	b.live = live
	b.liveOrder = order
	b.mu.Unlock()
	return nil
}

func (b *board) Jobs() []JobView {
	b.mu.RLock()
	live, order := b.live, b.liveOrder
	b.mu.RUnlock()

	declared := make(map[string]bool, len(b.jobs))
	views := make([]JobView, 0, len(b.jobs))
	for _, job := range b.jobs {
		declared[job.Name] = true
		views = append(views, b.own(declaredView(job, live[job.Name])))
	}

	// A job the daemon holds but run.toml no longer declares is still running:
	// hiding it would leave no way to read or stop it from here.
	for _, name := range order {
		if declared[name] {
			continue
		}
		views = append(views, b.own(undeclaredView(live[name])))
	}
	return views
}

// own stamps a view with the worktree it came from, which is what lets a merged
// board tell two jobs of the same name apart.
func (b *board) own(view JobView) JobView {
	view.WorkDir = b.workDir
	view.Worktree = b.worktree
	return view
}

func (b *board) Attach(params AttachParams) (Stream, error) {
	view, found := b.view(params.Job)
	if !found {
		return nil, fmt.Errorf("%w: %s", domain.ErrJobNotFound, params.Job)
	}
	if !view.Attachable {
		return nil, fmt.Errorf("%w: %s", domain.ErrJobNotAttachable, params.Job)
	}
	return b.service.Attach(AttachRequest{Name: params.Job, WorkDir: b.workDir, Size: params.Size})
}

func (b *board) History(params HistoryParams) ([]string, error) {
	lines := params.Lines
	if lines <= 0 {
		lines = domain.JobLogTailLines
	}
	return b.service.Tail(TailRequest{LogDir: b.logDir, Job: params.Job, Lines: lines})
}

func (b *board) view(name string) (JobView, bool) {
	for _, view := range b.Jobs() {
		if view.Name == name {
			return view, true
		}
	}
	return JobView{}, false
}

func declaredView(job domain.JobConfig, info domain.JobInfo) JobView {
	view := JobView{Name: job.Name, Kind: job.Kind, Status: domain.JobStatusStopped}
	if info.Name != "" {
		view.Status = info.Status
		view.StartedAt = info.StartedAt
		view.ExitCode = info.ExitCode
	}
	view.Attachable = view.Status == domain.JobStatusRunning && !rules.IsDetached(job)
	return view
}

// undeclaredView describes a job whose config is gone. Nothing says whether the
// service was a detached launcher, so the stream is offered and the daemon has
// the last word on refusing it.
func undeclaredView(info domain.JobInfo) JobView {
	return JobView{
		Name:       info.Name,
		Kind:       info.Kind,
		Status:     info.Status,
		StartedAt:  info.StartedAt,
		ExitCode:   info.ExitCode,
		Attachable: info.Status == domain.JobStatusRunning,
	}
}
