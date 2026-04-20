package domain

import "fmt"

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

// IsDetached reports whether the job is a service with a stop command,
// meaning the launcher process exits after starting detached work
// (e.g. docker compose up -d).
func (j JobConfig) IsDetached() bool {
	return j.Kind == JobKindService && j.Stop != ""
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

// DefaultProfile returns the profile marked as default, or the first one.
func (c RunConfig) DefaultProfile() (ProfileConfig, bool) {
	for _, p := range c.Profiles {
		if p.Default {
			return p, true
		}
	}
	if len(c.Profiles) > 0 {
		return c.Profiles[0], true
	}
	return ProfileConfig{}, false
}

// FindProfile returns a profile by name.
func (c RunConfig) FindProfile(name string) (ProfileConfig, bool) {
	for _, p := range c.Profiles {
		if p.Name == name {
			return p, true
		}
	}
	return ProfileConfig{}, false
}

// FindJob returns a job by name.
func (c RunConfig) FindJob(name string) (JobConfig, bool) {
	for _, j := range c.Jobs {
		if j.Name == name {
			return j, true
		}
	}
	return JobConfig{}, false
}

// ProfileJobs returns the JobConfig list for a profile, preserving the
// declared order.
func (c RunConfig) ProfileJobs(profile ProfileConfig) []JobConfig {
	var jobs []JobConfig
	for _, name := range profile.Jobs {
		if j, ok := c.FindJob(name); ok {
			jobs = append(jobs, j)
		}
	}
	return jobs
}

// FilterToProfile returns a new RunConfig containing only the named profile
// and the jobs it references. Returns an error if the profile is not found.
func (c RunConfig) FilterToProfile(name string) (RunConfig, error) {
	p, ok := c.FindProfile(name)
	if !ok {
		return RunConfig{}, fmt.Errorf("profile %q not found", name)
	}
	return RunConfig{
		Jobs:     c.ProfileJobs(p),
		Profiles: []ProfileConfig{p},
	}, nil
}
