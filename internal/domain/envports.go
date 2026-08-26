package domain

// EnvPortLink is one [[env_port]] entry of run.toml: the .env key whose value
// carries the host port of a declared job port. The link names the key only —
// where the port sits inside that value is found by looking for the declared
// base, so a bare port and a port buried in a URL are the same case.
type EnvPortLink struct {
	File string `toml:"file" json:"file"`
	Key  string `toml:"key"  json:"key"`
	// Job names which job's port the key follows. Two jobs may each declare a
	// PORT — their environments are separate — so the port name alone does not
	// identify a base. Every port belongs to exactly one job, so this is always
	// knowable and always required.
	Job  string `toml:"job"  json:"job"`
	Port string `toml:"port" json:"port"`
}

// PortRef identifies one declared port: the job that carries it and its name.
type PortRef struct {
	Job  string
	Name string
}

// EnvPortStatus is the verdict for one link against the value the .env holds.
// Only EnvPortStatusRewrite is ever written back: the rest are reported, because
// guessing which number of a URL is the port can corrupt it.
type EnvPortStatus string

const (
	// EnvPortStatusRewrite means the base was found exactly once and the value changes.
	EnvPortStatusRewrite EnvPortStatus = "rewrite"
	// EnvPortStatusUnchanged means the value already holds the resolved port — the
	// main worktree, whose offset is zero, or a file reconciled twice.
	EnvPortStatusUnchanged EnvPortStatus = "unchanged"
	// EnvPortStatusMissingKey means the .env has no such key.
	EnvPortStatusMissingKey EnvPortStatus = "missing_key"
	// EnvPortStatusNotFound means neither the base nor the resolved port appears in
	// the value; nothing anchors the substitution.
	EnvPortStatusNotFound EnvPortStatus = "base_not_found"
	// EnvPortStatusAmbiguous means the port appears more than once in the value.
	EnvPortStatusAmbiguous EnvPortStatus = "ambiguous"
)

// EnvPortEntry is one link resolved against the worktree's offset and the value
// currently in the file. NewValue is meaningful only for EnvPortStatusRewrite.
type EnvPortEntry struct {
	File         string        `json:"file"`
	Key          string        `json:"key"`
	Port         string        `json:"port"`
	Base         int           `json:"base"`
	Resolved     int           `json:"resolved"`
	Status       EnvPortStatus `json:"status"`
	CurrentValue string        `json:"current_value,omitempty"`
	NewValue     string        `json:"new_value,omitempty"`
}

// EnvPortPlan is every link of a worktree resolved at once, entries ordered as
// run.toml declares them.
type EnvPortPlan struct {
	Offset  int            `json:"offset"`
	Entries []EnvPortEntry `json:"entries"`
	// Applied says the rewrites were written. A plan that was only computed — a
	// --check run, or one the user declined — carries them all the same, and
	// counting those as written would be a false report.
	Applied bool `json:"applied"`
}

// Rewrites returns the entries that would change a value.
func (p EnvPortPlan) Rewrites() []EnvPortEntry {
	out := make([]EnvPortEntry, 0, len(p.Entries))
	for _, e := range p.Entries {
		if e.Status == EnvPortStatusRewrite {
			out = append(out, e)
		}
	}
	return out
}

// Anomalies returns the entries wtm refuses to act on and reports instead.
func (p EnvPortPlan) Anomalies() []EnvPortEntry {
	out := make([]EnvPortEntry, 0, len(p.Entries))
	for _, e := range p.Entries {
		switch e.Status {
		case EnvPortStatusMissingKey, EnvPortStatusNotFound, EnvPortStatusAmbiguous:
			out = append(out, e)
		}
	}
	return out
}
