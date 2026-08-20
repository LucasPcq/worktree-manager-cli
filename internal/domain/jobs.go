package domain

import "time"

// JobKind distinguishes a long-running service from a one-shot task.
type JobKind string

const (
	// JobKindService runs long-running. With a Stop command, it is treated as
	// detached (docker compose up -d pattern); without one, tracked by PID
	// and killed via SIGTERM.
	JobKindService JobKind = "service"

	// JobKindTask is a one-shot script that must exit 0 before the profile
	// continues. Output is streamed live, the job is removed on exit.
	JobKindTask JobKind = "task"
)

// JobConfig defines a managed job from .wtm/run.toml.
type JobConfig struct {
	Name string  `toml:"name"           json:"name"`
	Kind JobKind `toml:"kind"           json:"kind"`
	Cmd  string  `toml:"cmd"            json:"cmd"`
	Stop string  `toml:"stop,omitempty" json:"stop,omitempty"`
	Cwd  string  `toml:"cwd,omitempty"  json:"cwd,omitempty"`
}

// ProfileConfig defines a named, ordered group of jobs.
type ProfileConfig struct {
	Name    string   `toml:"name"    json:"name"`
	Jobs    []string `toml:"jobs"    json:"jobs"`
	Default bool     `toml:"default" json:"default"`
}

// RunConfig is the top-level structure of .wtm/run.toml. Each [[job]] and
// [[profile]] block declares one entry; the Go field names are kept plural
// because they hold the slices of all entries.
type RunConfig struct {
	Jobs     []JobConfig     `toml:"job"              json:"job"`
	Profiles []ProfileConfig `toml:"profile,omitempty" json:"profile"`
}

// JobStatus represents the current state of a managed job.
type JobStatus string

const (
	JobStatusRunning JobStatus = "running"
	JobStatusStopped JobStatus = "stopped"
	JobStatusCrashed JobStatus = "crashed"
)

// JobInfo is the JSON representation of a managed job, shared across the daemon
// protocol and the output/tui layers that render it.
type JobInfo struct {
	Name    string    `json:"name"`
	Kind    JobKind   `json:"kind"`
	Status  JobStatus `json:"status"`
	PID     int       `json:"pid"`
	WorkDir string    `json:"work_dir"`
	// StartedAt is when the daemon spawned the process. Zero for a job it never
	// spawned — one named by a picker, or read back after a daemon restart.
	StartedAt time.Time `json:"started_at,omitzero"`
	// ExitCode stays nil until the job's own process is reaped, and -1 says a
	// signal killed it. A detached launcher exiting does not end its job, so it
	// keeps a nil code for as long as the service it started is registered.
	ExitCode *int `json:"exit_code,omitempty"`
}

// LogRecord is one sanitized line of a job's output, as persisted in that job's
// log file.
type LogRecord struct {
	At   time.Time
	Text string
}
