package seam

import (
	"context"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
)

type SetParams struct {
	ProjectDir string
	StateDir   string
	// WorkDirs are the worktrees this run covers, as git spells them, in the
	// order they were selected. That order is what every listing and every recap
	// reads back.
	WorkDirs []string
	// Jobs are what the board lists, shared by every worktree in the set:
	// run.toml lives in the common git dir, so the worktrees of a repository
	// declare the same jobs and the same profiles.
	Jobs        []domain.JobConfig
	ProxyPort   int
	ProbeBudget time.Duration
	NoProbe     bool
}

// Set is the seam over several worktrees at once. It holds one Seam each and
// adds exactly two things: a board that shows them as one, and a start sequence
// that runs them concurrently.
type Set struct {
	seams []Seam
}

func OpenSet(params SetParams) Set {
	seams := make([]Seam, 0, len(params.WorkDirs))
	for _, workDir := range params.WorkDirs {
		seams = append(seams, Open(Params{
			ProjectDir:  params.ProjectDir,
			StateDir:    params.StateDir,
			WorkDir:     workDir,
			Jobs:        params.Jobs,
			ProxyPort:   params.ProxyPort,
			ProbeBudget: params.ProbeBudget,
			NoProbe:     params.NoProbe,
		}))
	}
	return Set{seams: seams}
}

func (s Set) Board() runlogs.Board {
	entries := make([]runlogs.MergedEntry, 0, len(s.seams))
	for _, seam := range s.seams {
		entries = append(entries, runlogs.MergedEntry{WorkDir: seam.workDir, Board: seam.Board()})
	}
	return runlogs.NewMergedBoard(entries)
}

func (s Set) Worktrees() []string {
	names := make([]string, 0, len(s.seams))
	for _, seam := range s.seams {
		names = append(names, seam.Worktree())
	}
	return names
}

// Starter runs every worktree's sequence concurrently and reports them all to
// one Sink. Concurrently is the whole point of the isolation: each worktree has
// its own ports and its own resource names, so starting one after another would
// only make a batch slower than two terminals.
//
// A worktree that aborts does not touch the others. They are isolated by
// construction, and an isolation that propagates a failure is not one — only the
// exit code the command owes its caller reads the set as a whole.
func (s Set) Starter(params StartParams) runlogs.StartFunc {
	return func(ctx context.Context, sink runlogs.Sink) (runlogs.Outcomes, error) {
		if len(s.seams) == 1 {
			return s.seams[0].Starter(params)(ctx, sink)
		}

		serialized := &lockedSink{sink: sink}
		outcomes := make(runlogs.Outcomes, len(s.seams))
		errs := make([]error, len(s.seams))

		var wg sync.WaitGroup
		for index, seam := range s.seams {
			wg.Add(1)
			go func() {
				defer wg.Done()
				outcomes[index], errs[index] = seam.run(ctx, serialized, params)
			}()
		}
		wg.Wait()

		return outcomes, firstError(errs)
	}
}

func firstError(errs []error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// lockedSink upholds runlogs' contract that a Sink is emitted to from one
// goroutine at a time. N sequences report to the surface that opened them, and
// a surface has no reason to learn how many wrote to it.
type lockedSink struct {
	mu   sync.Mutex
	sink runlogs.Sink
}

func (l *lockedSink) Emit(event runlogs.Event) {
	if l.sink == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sink.Emit(event)
}
