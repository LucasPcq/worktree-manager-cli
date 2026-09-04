package dashboard

import "github.com/LucasPcq/wtm/internal/flow"

// operation is a flow the dashboard started and has not seen finish. Its
// targets are the worktrees it works on, known only once the session naming
// them is answered — several since the run flows became cumulative. stage is
// the presenter's latest Stage/HookPhase message — what the list shows in place
// of a locked row's state pill.
type operation struct {
	id      int
	kind    string
	mode    flow.Mode
	targets []string
	stage   string
}

// operations is what the dashboard has in flight. A run declares how it holds the
// surface (flow.Operation); this is where that declaration is enforced, once,
// instead of at every action site.
type operations struct {
	running []operation
	lastID  int
}

func (o operations) begin(op operation) (operations, int) {
	o.lastID++
	op.id = o.lastID
	o.running = append(append([]operation(nil), o.running...), op)
	return o, op.id
}

func (o operations) retarget(id int, targets []string) operations {
	running := append([]operation(nil), o.running...)
	for index := range running {
		if running[index].id == id {
			running[index].targets = targets
		}
	}
	o.running = running
	return o
}

// stage records the run's latest Stage/HookPhase message — the text a locked
// row shows in place of its state pill while the operation holding it works.
func (o operations) stage(id int, text string) operations {
	running := append([]operation(nil), o.running...)
	for index := range running {
		if running[index].id == id {
			running[index].stage = text
		}
	}
	o.running = running
	return o
}

// holding reports the run that owns a worktree. A background run keeps its
// target to itself: nothing else may act on a worktree still being worked on.
func (o operations) holding(target string) (operation, bool) {
	if target == "" {
		return operation{}, false
	}
	for _, op := range o.running {
		for _, held := range op.targets {
			if held == target {
				return op, true
			}
		}
	}
	return operation{}, false
}

// active reports whether any run is still in flight. The spinner loop reads it:
// a locked row's glyph must keep advancing for the whole run, or it reads as
// hung rather than as busy.
func (o operations) active() bool { return len(o.running) > 0 }

// blocking reports a run that holds the whole surface, and with it every action.
func (o operations) blocking() (operation, bool) {
	for _, op := range o.running {
		if op.mode == flow.ModeBlocking {
			return op, true
		}
	}
	return operation{}, false
}

func (o operations) byID(id int) (operation, bool) {
	for _, op := range o.running {
		if op.id == id {
			return op, true
		}
	}
	return operation{}, false
}

func (o operations) end(id int) operations {
	running := make([]operation, 0, len(o.running))
	for _, op := range o.running {
		if op.id != id {
			running = append(running, op)
		}
	}
	o.running = running
	return o
}
