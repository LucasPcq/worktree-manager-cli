package domain

// ProxyRoute is one entry of the run proxy's table: a hostname, and the loopback
// address of the job answering under it. It is a projection of a running job,
// never a second source of truth.
type ProxyRoute struct {
	Host     string `json:"host"`
	Target   string `json:"target"`
	Job      string `json:"job"`
	Worktree string `json:"worktree"`
	Project  string `json:"project"`
}

// ProxyConfig is the [proxy] table of ~/.config/wtm/config.toml. The port is a
// property of the machine, not of a repository: the daemon is global and serves
// every repo at once.
type ProxyConfig struct {
	Port int `toml:"port,omitempty" json:"port,omitempty"`
	// Enabled is a pointer for the same reason UIConfig.Animations is: absent
	// and explicitly false are different answers.
	Enabled *bool `toml:"enabled,omitempty" json:"enabled,omitempty"`
}
