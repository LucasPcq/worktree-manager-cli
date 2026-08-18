package dashboard

import "github.com/LucasPcq/wtm/internal/flow"

// operation is a flow the dashboard started and has not seen finish. Its target
// is the worktree it works on, known only once the session naming it is answered.
type operation struct {
	id     int
	kind   string
	mode   flow.Mode
	target string
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

func (o operations) retarget(id int, target string) operations {
	running := append([]operation(nil), o.running...)
	for index := range running {
		if running[index].id == id {
			running[index].target = target
		}
	}
	o.running = running
	return o
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
