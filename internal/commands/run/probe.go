package run

import (
	"context"
	"time"

	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/service/portprobe"
)

// dialProber is the surface's side of the runlogs.Prober seam: the flow says
// which ports to check, the surface owns the budget and the socket.
type dialProber struct{ budget time.Duration }

func (p dialProber) Listening(ctx context.Context, ports []int, settled func(map[int]bool) bool) map[int]bool {
	return portprobe.Poll(ctx, portprobe.PollParams{Ports: ports, Budget: p.budget, Settled: settled})
}

// newProber returns nil when the check is switched off, which is what the flow
// reads to skip it entirely.
func newProber(budget time.Duration, disabled bool) runlogs.Prober {
	if disabled || budget <= 0 {
		return nil
	}
	return dialProber{budget: budget}
}
