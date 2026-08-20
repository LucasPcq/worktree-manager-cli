package runlogs

import (
	"fmt"
	"sync"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type SessionParams struct {
	Service Service
	// Jobs are the worktree's declared jobs, in the order a surface lists them.
	Jobs    []domain.JobConfig
	WorkDir string
	LogDir  string
}

// NewSession builds the surface's view of a worktree's jobs. It reads nothing
// until Refresh: a surface opens before the daemon is asked anything, and a job
// nobody has started yet is still a job to show.
func NewSession(params SessionParams) Session {
	return &session{
		service: params.Service,
		jobs:    params.Jobs,
		workDir: params.WorkDir,
		logDir:  params.LogDir,
	}
}

type session struct {
	service Service
	jobs    []domain.JobConfig
	workDir string
	logDir  string

	// mu guards live, which a surface refreshes off the goroutine that renders it.
	mu        sync.RWMutex
	live      map[string]domain.JobInfo
	liveOrder []string
}

func (s *session) Refresh() error {
	infos, err := s.service.List(s.workDir)
	if err != nil {
		return err
	}

	live := make(map[string]domain.JobInfo, len(infos))
	order := make([]string, 0, len(infos))
	for _, info := range infos {
		if info.WorkDir != s.workDir {
			continue
		}
		if _, known := live[info.Name]; !known {
			order = append(order, info.Name)
		}
		live[info.Name] = info
	}

	s.mu.Lock()
	s.live = live
	s.liveOrder = order
	s.mu.Unlock()
	return nil
}

func (s *session) Jobs() []JobView {
	s.mu.RLock()
	live, order := s.live, s.liveOrder
	s.mu.RUnlock()

	declared := make(map[string]bool, len(s.jobs))
	views := make([]JobView, 0, len(s.jobs))
	for _, job := range s.jobs {
		declared[job.Name] = true
		views = append(views, declaredView(job, live[job.Name]))
	}

	// A job the daemon holds but run.toml no longer declares is still running:
	// hiding it would leave no way to read or stop it from here.
	for _, name := range order {
		if declared[name] {
			continue
		}
		views = append(views, undeclaredView(live[name]))
	}
	return views
}

func (s *session) Attach(params AttachParams) (Stream, error) {
	view, found := s.view(params.Job)
	if !found {
		return nil, fmt.Errorf("%w: %s", domain.ErrJobNotFound, params.Job)
	}
	if !view.Attachable {
		return nil, fmt.Errorf("%w: %s", domain.ErrJobNotAttachable, params.Job)
	}
	return s.service.Attach(AttachRequest{Name: params.Job, WorkDir: s.workDir, Size: params.Size})
}

func (s *session) History(params HistoryParams) ([]string, error) {
	lines := params.Lines
	if lines <= 0 {
		lines = domain.JobLogTailLines
	}
	return s.service.Tail(TailRequest{LogDir: s.logDir, Job: params.Job, Lines: lines})
}

func (s *session) view(name string) (JobView, bool) {
	for _, view := range s.Jobs() {
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
