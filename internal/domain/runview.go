package domain

// RunViewLayout is the resolved geometry of one run-view frame: where the job
// list, the selected job's pane and the chrome around them sit. The renderer
// draws from it and the emulator is sized from it, so a pane cannot be drawn at
// one size and fed at another.
type RunViewLayout struct {
	Header  Rect
	Notice  Rect
	Sidebar Rect
	Pane    Rect
	Help    Rect

	// SidebarVisible is false on a terminal too narrow to name a job beside its
	// output; the pane then takes the whole width.
	SidebarVisible bool
	// SidebarRows is how many job lines fit in the list's body.
	SidebarRows int
	// PaneCols and PaneRows size the terminal emulator behind the pane box —
	// what is left once its border and title row are taken out.
	PaneCols int
	PaneRows int
}

// JobStep is where a job stands in a profile's start sequence, which the
// daemon's own view of it lags behind: a job is marked started here before a
// list call says it is running.
type JobStep int

const (
	JobStepStarting JobStep = iota
	JobStepStarted
	JobStepDone
	JobStepFailed
)

// JobMark is the state a job wears in the run view's list, once the sequence's
// account of it and the daemon's have been reconciled.
type JobMark int

const (
	JobMarkStopped JobMark = iota
	JobMarkStarting
	JobMarkRunning
	JobMarkDone
	JobMarkCrashed
)
