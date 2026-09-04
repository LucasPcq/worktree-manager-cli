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
	// ByDir says the link was attached from the directory holding the file
	// rather than from a value carrying the declared base. It never reaches
	// run.toml — it only marks the proposal, so the reader can tell a match
	// from a deduction before confirming.
	ByDir bool `toml:"-" json:"-"`
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
	// It cannot arise on an origin rewrite: replacing an authority is structural,
	// so a port sitting in a path or a query is never a candidate.
	EnvPortStatusAmbiguous EnvPortStatus = "ambiguous"
	// EnvPortStatusForeignHost means the value is a URL pointing somewhere the
	// proxy does not serve — a staging host, say. The link names a local job, so
	// the two disagree and wtm reports rather than picks.
	EnvPortStatusForeignHost EnvPortStatus = "foreign_host"
	// EnvPortStatusSecureScheme means the value is https and the proxy speaks
	// plain HTTP. Downgrading a scheme the user wrote on purpose is not wtm's
	// call, and an app serving https locally already has its own CA.
	EnvPortStatusSecureScheme EnvPortStatus = "secure_scheme"
)

// EnvPortMove is one port a key follows: what it is declared at, and what this
// worktree binds it to.
type EnvPortMove struct {
	// Port names the declaration the move follows, so a key following two jobs
	// says which is which.
	Port     string `json:"port"`
	Job      string `json:"job"`
	Base     int    `json:"base"`
	Resolved int    `json:"resolved"`
}

// EnvPortEntry is one link resolved against the worktree's offset and the value
// currently in the file. NewValue is meaningful only for EnvPortStatusRewrite.
type EnvPortEntry struct {
	File string `json:"file"`
	Key  string `json:"key"`
	Port string `json:"port"`
	// Base and Resolved are the first port the key follows. A value holding a
	// list of origins follows several — Moves carries them all, this pair its
	// first, which is every reading written before a key could follow more than
	// one.
	Base     int           `json:"base"`
	Resolved int           `json:"resolved"`
	Moves    []EnvPortMove `json:"moves,omitempty"`
	// Addressing is how this one entry was resolved, not what the project asked
	// for: a project on AddressingNames still resolves its bare-port links by
	// port, and the table has to render the two differently.
	Addressing   Addressing    `json:"addressing"`
	Status       EnvPortStatus `json:"status"`
	CurrentValue string        `json:"current_value,omitempty"`
	NewValue     string        `json:"new_value,omitempty"`
	// ForeignHost is what the value pointed at when it pointed somewhere the
	// proxy does not serve. Only EnvPortStatusForeignHost carries it.
	ForeignHost string `json:"foreign_host,omitempty"`
}

// EnvPortPlan is every link of a worktree resolved at once, entries ordered as
// run.toml declares them.
type EnvPortPlan struct {
	Offset  int            `json:"offset"`
	Entries []EnvPortEntry `json:"entries"`
	// Addressing is what the project asked for, which is not always what the
	// entries got: a machine with no proxy writes ports whatever run.toml says.
	Addressing Addressing `json:"addressing"`
	// PublicPort is what an address announces here, zero when nothing serves
	// names. Both are what the notices are derived from.
	PublicPort int `json:"public_port,omitempty"`
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
		case EnvPortStatusMissingKey, EnvPortStatusNotFound, EnvPortStatusAmbiguous,
			EnvPortStatusForeignHost, EnvPortStatusSecureScheme:
			out = append(out, e)
		}
	}
	return out
}
