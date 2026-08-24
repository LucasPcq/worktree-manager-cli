package domain

// PortProbeStatus is what a probe found on the port a job declared. It says
// what was observed, never what is at fault: wtm can see that nothing answers,
// not why.
type PortProbeStatus string

const (
	PortListening PortProbeStatus = "listening"
	PortSilent    PortProbeStatus = "silent"
)

// PortProbe is one declared port checked after its job started.
type PortProbe struct {
	Job  string
	Name string
	// Port is the resolved port, base plus this worktree's offset.
	Port   int
	Status PortProbeStatus
	// BaseListening is the base port when it answers and the resolved one does
	// not — the signature of a variable that never reached the process. Zero
	// otherwise, and always zero on the main checkout, where base and resolved
	// are the same port and there is nothing to tell apart.
	BaseListening int
}
