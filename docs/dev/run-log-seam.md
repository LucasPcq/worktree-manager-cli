# `internal/flow/runlogs` — the run log seam

`run up`, `run start` and `run logs` are the first commands with **two surfaces** that
are not the wizard: a full-screen terminal view and a stream of lines. They do not go
through the `flow.Step` vocabulary — a start sequence asks no questions — but they obey
the same rule that made it worth having: *the surface is not where the run lives*.

This document describes the code as delivered.

- [What the seam is](#what-the-seam-is)
- [The two surfaces](#the-two-surfaces)
- [Choosing one](#choosing-one)
- [Detaching, and what it cancels](#detaching-and-what-it-cancels)
- [Why the outcome carries the failed job's output](#why-the-outcome-carries-the-failed-jobs-output)
- [What is not in the seam yet](#what-is-not-in-the-seam-yet)

## What the seam is

`internal/flow/runlogs` sits under `internal/flow/` and imports what a flow may import:
`service/`, `rules/`, `domain/`, the stdlib. It never formats, colours or frames
anything.

| Type | What it answers |
| -- | -- |
| `Service` | the daemon as this package uses it — `Start`, `List`, `Attach`, `Tail`. `NewService` implements it over `internal/service/process`; `runlogstest.Service` replaces it in tests |
| `Session` | the surface's view of a worktree: `Jobs() []JobView`, `Refresh`, `Attach`, `History` |
| `Stream` | one attached job: `Chunks()` (raw bytes, escape sequences included), `Write` (stdin), `Resize`, `Close` |
| `Run(ctx, RunParams)` | a profile's start sequence — declared order, tasks blocking, a failure ending it — emitting `Event`s to a `Sink` |
| `Outcome` | what the sequence left behind: `Started`, `Completed`, `NotStarted`, `Failed`/`FailedStep`, `FailedOutput`, `FailedExitCode` |

`Sink`/`Event` is the `Presenter` of this seam: one value per phase (`PhaseStarting`,
`PhaseOutput`, `PhaseStarted`, `PhaseDone`, `PhaseFailed`, `PhaseAborted`, `PhaseReady`),
rendered by whoever installed the sink.

## The two surfaces

| | Full-screen | Line by line |
| -- | -- | -- |
| Where | `internal/tui/runview` | `internal/commands/run/surface.go` (`textSink`) |
| A job's output | a pane backed by a VT emulator, escape sequences and all | sanitized by `rules.SanitizeLogChunk`, one whole line at a time |
| A phase | the job list's marks, the step counter, the abort report | `output.Loading` / `output.Success` / `output.Error` |
| The conclusion | `Result.Recap`, a raw body the command frames | the abort report, or the JSON document built from `Outcome.Results` |

Both are handed the same `surfaceParams`, and both call the same `runlogs.Run`. The view
is opened **before** the first job starts: a task that scrolls has a pane to scroll in,
and the abort report is rendered where the panes are rather than underneath them.

## Choosing one

`rules.UseRunView` is the whole decision, and `internal/commands/run/surface.go`
(`wantsRunView`) only adds the terminal to it:

```go
func UseRunView(params RunSurfaceParams) bool {
	if params.Detach || params.Inline || !params.TTY {
		return false
	}
	return IsHumanFormat(params.Format)
}
```

`Inline` is `rules.RunsInline(job)` — a task owns the terminal while it runs, so
`run start <task>` streams into the scrollback and never opens the view, with or without
`-d`. This is the run module's reading of the two bypass axes: `-d` is the confirmation
axis (the surface the caller is willing to give up), and there is no safety axis here —
nothing a run refuses is lifted by a flag.

## Detaching, and what it cancels

`Run` takes a context, and cancelling it ends the **reporting**, never the jobs. Leaving
the view (`q`, `Ctrl+C` outside focus) cancels it: the sequence stops where it stands,
the jobs already started stay up, the ones it never reached come back as `NotStarted`,
and `emit` drops what it is handed rather than posting to a surface that is gone.

## Why the outcome carries the failed job's output

`Outcome.FailedOutput` is the raw bytes the job that ended the sequence had written, and
`FailedExitCode` the code the daemon reported. A surface that showed them live does not
need them; every other one has nothing else. `rules.WithFailureOutput` folds them into
the failing `JobActionResult`, which is what makes

```json
[{"name": "docker", "status": "started"},
 {"name": "migrate", "status": "error",
  "message": "task migrate failed: exit status 1\nrelation \"users\" does not exist"}]
```

readable by an agent or a CI job that never saw the stream. Pinned by
`TestStartProfileInlineJSONCarriesTheFailedOutput`.

## What is not in the seam yet

- **`--exclusive` / `--parallel` and their prompt** (`handleConcurrentJobs` in
  `internal/commands/run/up.go`) are still the command's own decision, made before the
  seam is reached. Worktree isolation (LUC-99/100) removes the question rather than
  moving it: migrating it into `runlogs` first would be work to delete.
- **No `Prompter`.** A start sequence asks nothing. The day it does — the concurrency
  question is the candidate — it becomes a `flow.Step` whose `Resolve` names the flag
  that answers it unattended, and this package gains the seam the wizard flows have.
- **`run down` / `run stop` / `run ps` / `run list`** still talk to `process.Client`
  directly. They read or stop, they never stream, so nothing about them is waiting on
  this seam.
