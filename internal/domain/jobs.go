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

// JobURLConfig is a job's [[job]].url table: which of its declared ports speaks
// HTTP, and the host label it is published under. Port names a key of Ports, not
// a number — the number depends on the worktree, only the declaration is stable.
// Host is optional and defaults to the job's name.
type JobURLConfig struct {
	Port string `toml:"port"           json:"port"`
	Host string `toml:"host,omitempty" json:"host,omitempty"`
}

// JobConfig defines a managed job from .wtm/run.toml.
type JobConfig struct {
	Name string  `toml:"name"           json:"name"`
	Kind JobKind `toml:"kind"           json:"kind"`
	Cmd  string  `toml:"cmd"            json:"cmd"`
	Stop string  `toml:"stop,omitempty" json:"stop,omitempty"`
	Cwd  string  `toml:"cwd,omitempty"  json:"cwd,omitempty"`
	// Ports maps an environment variable to the port it takes on the main
	// checkout. Every other worktree gets that base plus its own offset, so the
	// same job binds a free port in each one.
	Ports map[string]int `toml:"ports,omitempty" json:"ports,omitempty"`
	// URL publishes one of the ports above under a name. Absent means the job
	// keeps no name and stays reachable by its port, as before.
	URL *JobURLConfig `toml:"url,omitempty" json:"url,omitempty"`
}

// JobURLEntry is one published job as a surface reports it: the job's name and
// where it answers in this worktree.
// JobURLChoice is one job's answer to "should this be reachable by name": the
// port to publish and whether to publish it. Publish false is an answer too — it
// withdraws a url the config already carried.
type JobURLChoice struct {
	Job     string
	Port    string
	Publish bool
}

type JobURLEntry struct {
	Job string `json:"job"`
	URL string `json:"url"`
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
	// PortOffsetBlock spaces two worktrees' declared ports apart. Zero means the
	// default: a project only sets it to make room for base ports that would
	// otherwise land a multiple of the block apart.
	PortOffsetBlock int `toml:"port_offset_block,omitempty" json:"port_offset_block,omitempty"`
	// PortProbeTimeout is how many seconds `run up` waits for a declared port to
	// answer before reporting it silent. Zero falls back to the default; a
	// negative value disables the check.
	PortProbeTimeout int             `toml:"port_probe_timeout,omitempty" json:"port_probe_timeout,omitempty"`
	Jobs             []JobConfig     `toml:"job"                        json:"job"`
	Profiles         []ProfileConfig `toml:"profile,omitempty"          json:"profile"`
	// EnvPorts links a .env key to one of the ports declared above, so a value
	// holding a hard-coded host port follows the worktree's offset.
	EnvPorts []EnvPortLink `toml:"env_port,omitempty" json:"env_port,omitempty"`
}

// ExecSpec is a command ready for exec: the binary and the arguments it takes,
// already resolved from whatever form the config wrote it in.
type ExecSpec struct {
	Name string
	Args []string
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
	// URL is where the job is reachable, absent for one that publishes no name.
	URL string `json:"url,omitempty"`
	// ExitCode stays nil until the job's own process is reaped, and -1 says a
	// signal killed it. A detached launcher exiting does not end its job, so it
	// keeps a nil code for as long as the service it started is registered.
	ExitCode *int `json:"exit_code,omitempty"`
}

// JobActionResult is one job's outcome as a `run *` command reports it, shared
// by every surface that speaks the JobAction* vocabulary — the JSON output and
// the flow seam alike.
type JobActionResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	// Message carries the detail when Status is JobActionError.
	Message string `json:"message,omitempty"`
	// Output is what the job had written when it failed, raw. A caller reading
	// this document never saw the live stream, and the daemon's message alone
	// ("task migrate failed: exit status 1") does not say why it stopped.
	Output string `json:"output,omitempty"`
	// ExitCode is what the failing job exited with, absent for one that never
	// got as far as running.
	ExitCode *int `json:"exit_code,omitempty"`
	// Ports is what the probe found on each port this job declared. A reader of
	// this document never saw the live stream, and "started" alone does not say
	// whether anything is listening.
	Ports []PortProbe `json:"ports,omitempty"`
	// URL is where the job is reachable, absent for one that publishes no name.
	URL string `json:"url,omitempty"`
}

// LogRecord is one sanitized line of a job's output, as persisted in that job's
// log file.
type LogRecord struct {
	At   time.Time
	Text string
}

// RunSurface names who shows a run's jobs: the full-screen view, a stream of
// lines on the terminal the command was launched from, or a machine-readable
// document.
type RunSurface int

const (
	RunSurfaceView RunSurface = iota
	RunSurfaceStream
	RunSurfaceMachine
)

// JobKindChoice is one job whose kind the wizard asks about. Label is how the
// job reads on screen ("apps/web / build"), Name the script it came from.
type JobKindChoice struct {
	Label string
	Cmd   string
	// Name and Workspace together identify the script: two packages of a
	// monorepo both declaring "build" are two separate answers.
	Name      string
	Workspace string
	Kind      JobKind
}

// DevOriginFix is a published Next job that would refuse requests arriving under
// its own name, and the line its config is missing.
type DevOriginFix struct {
	Job    string
	Config string
	Line   string
}

// JobCmdFix is a job whose command never mentions a port variable wtm injects
// for it. The command will bind whatever it binds today, ignoring the offset.
type JobCmdFix struct {
	Job  string
	Cmd  string
	Vars []string
}
