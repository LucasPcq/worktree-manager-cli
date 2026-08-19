# `internal/flow/` — how a command runs

A *flow* is everything a command does between "the flags are parsed" and "the result
is printed": the questions it asks, in what order, which ones it may skip, the safety
checks, the service calls, the phases it reports. It is written once, in
`internal/flow/<command>/`, and three different surfaces can run it.

This document describes the code **as delivered**. Where it says *projected*, nothing
is implemented yet.

- [What is delivered today](#what-is-delivered-today)
- [The shape of a flow](#the-shape-of-a-flow)
- [The three seams](#the-three-seams)
- [The step model](#the-step-model)
- [Command flow diagrams](#command-flow-diagrams)
- [One flow, three surfaces](#one-flow-three-surfaces)
- [Unattended resolution and the two axes](#unattended-resolution-and-the-two-axes)
- [Hook output, from the service writer to the dashboard panel](#hook-output-from-the-service-writer-to-the-dashboard-panel)
- [Testing a flow](#testing-a-flow)
- [Known gaps](#known-gaps)

## What is delivered today

| | Status |
| -- | -- |
| `wtm create` | migrated — `internal/flow/create` |
| `wtm clean` | migrated — `internal/flow/clean` |
| `wtm reparent` | migrated — `internal/flow/reparent` |
| `wtm prune` | migrated — `internal/flow/prune` |
| CLI wizard surface | `internal/tui/flowui` |
| Unattended surface | `flow.Unattended` (in `internal/flow`) |
| Dashboard surface | `internal/tui/dashboard` (`prompter.go`, `presenter.go`, `ops.go`) |
| Test doubles | `internal/testutil/flowtest` |
| `extract`, `sync` | **not migrated** — still driven by `internal/commands/wt` plus their wizard packages. The model was validated on paper against them; that is not the same as delivered. `extract` is LUC-182, `sync` is LUC-186. |
| `StepMultiSelect` | exists since `reparent`, which needed it to keep its no-argument picker. Rendered by both surfaces: `flowui`, and the dashboard's modal since its Actions menu runs the batch reparent. Since `prune`, an `Option` can also arrive pre-checked and tagged (`Selected`, `Tag`, `Tone`). |

## The shape of a flow

One package per command, splitting the run from the questions it asks:

```
internal/flow/create/
  create.go   the run: Request, Outcome, Presenter, Params, Run, Operation
  steps.go    the session: the flow.Step declarations and the recap
```

The entry point is always the same shape — one struct parameter, one outcome, one
error:

```go
type Params struct {
	Context   flow.Context   // ProjectDir, StateDir, Config
	Request   Request        // what the surface already knows
	Prompter  flow.Prompter  // who answers the questions
	Presenter Presenter      // where the phases go
}

func Run(params Params) (Outcome, error)
```

`Run` is a package-level function per command (`create.Run`, `clean.Run`), not a
method on a shared type. Behind it, an unexported `createFlow` / `cleanFlow` struct
holds the params so the step declarations can close over them.

**Errors are returned, never presented.** `flow` has no `Presenter.Error`: on the CLI
Cobra prints the error and `rules.ExitCode` sets the exit status; on the dashboard the
caller puts it in the output panel. A user abort is different — it is a `Notice`
followed by `Outcome{Aborted: true}` and a `nil` error, because the user cancelling is
not a failure.

## The three seams

### `flow.Prompter` — who answers

```go
type Prompter interface {
	Ask(Session) (Answers, error)
	Confirm(ConfirmParams) (bool, error)
	Interactive() bool
}
```

- **`Ask`** runs a whole question-and-recap sequence and returns every answer, keyed
  by `Step.Key`. It returns `domain.ErrUserAborted` when the user backs out.
- **`Confirm`** is a standalone decision that can only exist *after* an execution —
  a fast-forward that failed, a removal that needs `sudo`. There is no session left to
  join at that point.
- **`Interactive`** reports whether a decision may be offered at all. It is read for
  exactly two purposes: not offering a decision nobody can answer, and feeding a pure
  rule that takes it as an input (`rules.DecidePush`). Any other use puts the bypass
  taxonomy back into the commands, which is what this layer removes.

```go
type Session struct {
	ErrLabel string   // what the host calls the command if a step errors
	Steps    []Step
	Presets  Answers  // values the request already carries
}
```

Three implementations:

| Implementation | Where | `Ask` | `Confirm` | `Interactive()` |
| -- | -- | -- | -- | -- |
| `flowui.Prompter` | `internal/tui/flowui` | `components.RunWizard` | `components.RunStandaloneConfirm` | `true` |
| `flow.Unattended` | `internal/flow/unattended.go` | resolves with no interaction | `false, nil` | `false` |
| `dashboard.prompter` | `internal/tui/dashboard` | a modal, over a channel | a one-question modal | `true` |

`Unattended` lives in `flow/` on purpose: it is the only implementation with no
surface dependency, and it carries the bypass taxonomy — which must exist once, not
once per surface.

### `flow.Presenter` — where the phases go

```go
type Presenter interface {
	Stage(StageParams) error         // one unit of work under a progress indicator
	HookPhase(HookPhaseParams) error // a titled hook section + the sink hooks stream into
	Notice(Notice)                   // concludes the run
	Status(Notice)                   // one line inside an ongoing phase
}
```

A flow never frames, never animates and never picks a stream — it says *what phase
this is*, the surface decides how it reads. `Notice` carries a `NoticeKind`
(`NoticeMessage`, `NoticeWarning`, `NoticeSuccess`); `flow.AbortedNotice` is the
shared "Aborted." value.

The conclusion is **typed per command**, not generic, so `flow/` never has to decide
on a format:

```go
// internal/flow/create
type Presenter interface {
	flow.Presenter
	Created(Outcome) error
}

// internal/flow/clean
type Presenter interface {
	flow.Presenter
	Cleaned(Outcome) error
}
```

The outcome carries data (`domain.CreateResult`, the reparent results, the path), never
text. The CLI turns it into `output.FormatCreateResult` or a JSON payload; the
dashboard turns it into a line in the output panel plus a refresh message.

It is both an event and a return value, and that is not indecision: a conclusion has to
be *emitted during* the run for a flow whose recap must appear before a later prompt,
and the caller needs the *value* for the JSON payload and the exit code.

### `Request` — what the surface already knows

The request is declared by the command's flow package, not by `flow/`:

```go
// internal/flow/create
type Request struct {
	Branch      string // positional arg
	From        string // --from
	EnvFrom     string // --env-from
	FastForward bool   // --ff
	IfNotExists bool   // --if-not-exists
}

// internal/flow/clean
type Request struct {
	Branch           string
	Force            bool // the safety axis
	ReparentChildren bool
	BaseBranch       string
	AllowPrivileged  bool // may this surface hand the terminal to sudo?
}
```

It holds **no `--yes` and no `--output`**. The confirmation axis is the Prompter that
was installed; the output format is the surface's business. `--force` *does* belong
there: it is the safety axis, a business input the service consumes
(`domain.CleanParams.Force`), not a dialogue capability.

`AllowPrivileged` is the same idea applied to a surface capability: the CLI owns the
terminal it prompts on, so it can hand it to `sudo`; the dashboard is holding that
terminal in alt-screen, so it sets `false` and names the way out instead.

## The step model

```go
type Step struct {
	Kind        StepKind // StepText | StepSelect | StepBranchSelect | StepRecap
	Key         string   // identifies the answer in Answers
	Label       string   // the step's name in the breadcrumb / summaries
	Title       string
	Description string
	Options     []Option

	Branches []domain.BranchCandidate // StepBranchSelect only
	Pinned   string
	Refresh  func() []domain.BranchCandidate

	Validate       func(value string) error
	Skip           func(Answers) (skip bool, reason string)
	Build          func(Answers) (StepContent, error) // re-derive content, synchronously
	Load           func(Answers) (StepContent, error) // same, but it does I/O
	LoadingMessage string

	Resolve   func(Answers) (Answer, error) // the whole bypass taxonomy, see below
	Summarize func(Answer) string
	Flag      string
}
```

`StepContent` is the part that may depend on earlier answers — `Title`, `Description`,
`Options`, and `Blockers`.

```go
// Blocker is one safety refusal standing in the way of the step's dangerous
// option, stated on its own so nothing is ever lifted implicitly.
type Blocker struct {
	Key   string
	Label string
}
```

Blockers are why the dashboard can offer a per-refusal acknowledgement where the CLI
prints a list: `rules.CleanBlockers` produces them, `internal/flow/clean/steps.go`
attaches them to the delete step, and the dashboard renders each as a checkbox that
must be ticked before the dangerous option becomes submittable. Folding them into the
prose would have made that impossible.

Answers are immutable and typed — no `any` anywhere:

```go
type Answer struct {
	Value      string
	Skipped    bool
	SkipReason string
	Asked      bool // false for a preset, a Resolve fallback, or a skip
}

func NewAnswers(values map[string]string) Answers        // "" means unanswered
func (a Answers) With(key string, answer Answer) Answers // returns a copy
func (a Answers) Get(key string) (Answer, bool)
func (a Answers) Value(key string) string
func (a Answers) Answered(key string) bool // asked, and not skipped
```

`Presets` is what keeps a flag from erasing a recap line: a preset step is **not
asked**, but it is still read back by the recap builder, so `wtm create feat/x --from
main` shows the same three lines as the fully interactive run. `Answered` is the
converse — it tells a flow whether a human actually saw a question, which is how a
recap that was auto-confirmed stays distinguishable from one that was reviewed.

### `flow.Operation` — how a flow is scheduled

```go
type Operation struct {
	Kind      string // domain.OpKindCreate, domain.OpKindClean
	Mode      Mode   // ModeBlocking | ModeBackground
	TargetKey string // the answer naming the worktree this run holds
}
```

This is what a flow declares about *how it is scheduled*, for a surface that runs
several at once. `Mode` says how long it holds that surface: `ModeBlocking` (`clean`,
which destroys its target) keeps it until the run ends; `ModeBackground` (`create`,
whose hooks can run long) gives it back and locks its target instead. `TargetKey`
names the answer carrying that target — known only once the step is answered, which is
why the dashboard's prompter posts an `opTargetMsg` as soon as the session returns.

The CLI ignores all of it: one run, one terminal. `internal/tui/dashboard/ops.go` is
where it is enforced, once, instead of at every action site.

## Command flow diagrams

### `wtm create` (delivered)

```mermaid
flowchart TD
  A["create.Run"] --> B{"--from names a known branch?"}
  B -- no --> ERR1["error: branch not found"]
  B -- yes --> C{"branch already checked out elsewhere?"}
  C -- "yes, without --if-not-exists" --> ERR2["error: worktree exists"]
  C -- no --> D["Prompter.Ask(session)"]
  D --> D1["branch name — StepText"]
  D1 --> D2["source branch — StepBranchSelect"]
  D2 --> D3["env strategy — StepSelect"]
  D3 --> D4["source update — StepSelect, skipped unless behind"]
  D4 --> D5["recap — StepRecap"]
  D5 --> E{"aborted?"}
  E -- yes --> F["Notice aborted, then Outcome aborted with a nil error"]
  E -- no --> G{"source update = fast-forward?"}
  G -- yes --> H["Stage: fast-forward, then Confirm on failure"]
  G -- no --> I["Stage: worktree.Create with SkipHooks"]
  H --> I
  I --> J{"already existed?"}
  J -- no --> K["HookPhase: on_create hooks"]
  J -- yes --> L["Presenter.Created"]
  K --> L
```

The create flow calls `worktree.Create` with `SkipHooks: true` and runs the hooks as
its own phase afterwards, so the hook output does not fight the creation progress
indicator for the terminal.

### `wtm clean` (delivered)

```mermaid
flowchart TD
  A["clean.Run"] --> B{"branch given and Prompter interactive?"}
  B -- yes --> C["Stage: worktree.Check"]
  C --> C1{"absent, or a parent worktree?"}
  C1 -- absent --> C2["Cleaned: already absent"]
  C1 -- parent --> C3["Notice warning, stop"]
  C1 -- neither --> D["Prompter.Ask(session)"]
  B -- no --> D
  D --> D1["worktree — StepSelect, preset by the positional arg"]
  D1 --> D2["reparent children — StepSelect, skipped when there are none"]
  D2 --> D3["delete — StepRecap, carrying the Blockers"]
  D3 --> E{"aborted?"}
  E -- yes --> F["Notice aborted, then Outcome aborted with a nil error"]
  E -- no --> G["stop services, HookPhase: on_clean hooks"]
  G --> H["Stage: worktree.Clean"]
  H --> I{"removal failed on permissions?"}
  I -- "yes, and AllowPrivileged" --> J["Confirm sudo removal, then worktree.ForceClean"]
  I -- no --> K["apply the reparent plan when it was answered yes"]
  J --> K
  K --> L["Presenter.Cleaned"]
```

`force` for the service call is `request.Force || answers.Value(KeyDelete) == "force"`:
the flag lifts the refusals up front, or the user lifts them in the recap by choosing
the dangerous option. Both routes converge on one value.

### `wtm extract` and `wtm sync` — projected, not delivered

Neither command runs on `flow/` today; both still drive `internal/tui/extract` and
`internal/tui/syncpicker` from `internal/commands/wt`. The model was validated on paper
against them before the layer was written, and the diagrams below are that validation —
what the migration is expected to look like, not what runs. `extract` is tracked by
**LUC-182**, and it is the migration that removes the
temporary duplication of create's step declarations (they exist twice today: as
`flow.Step` for `wtm create`, and as `components.Step` in `internal/tui/newwt` for the
sub-flow `extract` embeds).

```mermaid
flowchart TD
  A["extract.Run — projected"] --> B["Ask: source worktree — StepSelect"]
  B --> C["Ask: files — StepMultiSelect, Load from the chosen source"]
  C --> D["Ask: target worktree — StepSelect, plus a create-new row"]
  D --> E["Ask: the create sub-flow steps, gated on create-new"]
  E --> F["Ask: move or copy — StepSelect"]
  F --> G["Ask: recap — StepRecap"]
  G --> H["service: conflicting files for this selection"]
  H --> I{"conflicts?"}
  I -- none --> J["on-conflict = abort"]
  I -- "yes, --on-conflict set" --> K["use the flag value"]
  I -- "yes, not interactive" --> J
  I -- "yes, interactive" --> L["Confirm: resolve or abort"]
  J --> M["execute the extraction"]
  K --> M
  L --> M
```

The on-conflict decision stays *outside* the session on purpose: the conflict list
depends on the selection **and** on the state of the disk, so it can only be asked
after the recap — a post-execution `Confirm`, like create's failed fast-forward.

```mermaid
flowchart TD
  A["sync.Run — projected"] --> B["Ask: worktrees — StepMultiSelect, preset by args or --all"]
  B --> C["Ask: on conflict — StepSelect"]
  C --> D["Ask: recap — StepRecap whose Load is the sync plan"]
  D --> E["service: rebase the selection"]
  E --> F["Presenter: the plan, then what was rebased"]
  F --> G["rules.DecidePush"]
  G -- PushForce --> H["push"]
  G -- PushConfirm --> I["Confirm, then push"]
  G -- PushSkip --> J["done, nothing pushed"]
```

`sync` is also why the conclusion is a Presenter method and not only a return value:
its recap must be shown *before* the push prompt.

## One flow, three surfaces

The same `create.Run` call, on the three surfaces. Only the two seams change.

### CLI, interactive

```mermaid
sequenceDiagram
  participant Cmd as commands/wt/create.go
  participant Flow as flow/create.Run
  participant P as flowui.Prompter
  participant Pr as cliPresenter
  participant Svc as service/worktree

  Cmd->>Cmd: parse flags, LoadConfig
  Cmd->>Cmd: interactive = human and TTY and not --yes, so true
  Cmd->>Flow: Run with flowui.New and createPresenter
  Flow->>P: Ask(session)
  P->>P: components.RunWizard — breadcrumb, Esc steps back
  P-->>Flow: Answers, Asked true
  Flow->>Pr: Stage "Creating ..."
  Pr->>Svc: worktree.Create
  Svc-->>Pr: CreateResult
  Flow->>Pr: HookPhase on_create
  Pr->>Svc: RunCreateHooks with Output = cmd.ErrOrStderr
  Flow->>Pr: Created(outcome)
  Pr->>Pr: output.Frame and FormatCreateResult
```

### CLI, unattended — `--yes`, no TTY, or `--output json`

```mermaid
sequenceDiagram
  participant Cmd as commands/wt/create.go
  participant Flow as flow/create.Run
  participant P as flow.Unattended
  participant Pr as cliPresenter
  participant Svc as service/worktree

  Cmd->>Cmd: interactive = human and TTY and not --yes, so false
  Cmd->>Flow: Run with flow.Unattended and createPresenter
  Flow->>P: Ask(session)
  loop each step
    P->>P: preset, else Skip, else Resolve, else refuse naming the flag
  end
  P-->>Flow: Answers, Asked false
  Flow->>Pr: Stage "Creating ..."
  Pr->>Svc: worktree.Create
  Note over Pr: same presenter, no animation, JSON payload on stdout
  Flow->>Pr: Created(outcome)
```

Nothing about the flow changes — no branch anywhere in `create.Run` reads "am I
unattended". The only decision the command makes is *which Prompter to install*.

### Dashboard

```mermaid
sequenceDiagram
  participant UI as dashboard Model, UI goroutine
  participant G as flow goroutine
  participant Flow as flow/create.Run
  participant P as dashboard.prompter
  participant Pr as dashboard.createPresenter

  UI->>UI: busyReason, then beginOp with create.Operation
  UI->>G: a tea.Cmd starts the run
  G->>Flow: create.Run with the dashboard prompter and presenter
  Flow->>P: Ask(session)
  P->>UI: promptMsg carrying the session and a reply channel
  UI->>UI: open the modal, render one step at a time
  UI-->>P: promptReply with the answers
  P->>UI: opTargetMsg — the run now holds this branch
  P-->>Flow: Answers
  Flow->>Pr: Stage, HookPhase, Created
  Pr->>UI: one OutputLineMsg per line, then createdMsg
  G-->>UI: opDoneMsg
```

Two things make this safe rather than merely possible: the flow runs on **its own
goroutine**, and every single thing it produces reaches the model as a `tea.Msg` on the
dashboard's channel. The prompter blocks on a reply channel — from the flow's
perspective `Ask` is an ordinary blocking call — while the UI goroutine keeps
rendering.

## Unattended resolution and the two axes

`flow.Unattended.Ask` is the entire bypass taxonomy, in one loop:

```go
func (Unattended) Ask(session Session) (Answers, error) {
	answers := session.Presets
	for _, step := range session.Steps {
		if _, known := answers.Get(step.Key); known {
			continue // the flag or the positional arg already answered it
		}
		if step.Skip != nil {
			if skip, reason := step.Skip(answers); skip {
				answers = answers.With(step.Key, Answer{Skipped: true, SkipReason: reason})
				continue
			}
		}
		if step.Resolve == nil {
			return Answers{}, requiredErr(step) // interactive-only: refuse, naming the flag
		}
		answer, err := step.Resolve(answers)
		if err != nil {
			return Answers{}, err
		}
		answers = answers.With(step.Key, answer)
	}
	return answers, nil
}
```

`Resolve` is where the three documented cases live, declared **on the step**, next to
the question they answer:

| Case | How the step declares it | Example |
| -- | -- | -- |
| 1. Decision or confirmation with a safe default | `Resolve` returns an `Answer` | create's env step returns `""` (the config default); create's source-update step returns `ff` only when `--ff` was passed, else `keep`; clean's reparent step returns `orphan`; every recap returns its confirm value |
| 2. Required selection with no safe default | `Resolve` returns an **error naming the flag** | create's source step refuses to guess the parent of a pre-existing branch and says to pass `--from`; clean's worktree step returns `domain.ErrCleanBranchRequired` |
| 3. Interactive-only | **no `Resolve` at all** | `Unattended` refuses with `requiredErr(step)`, built from `step.Label` and `step.Flag`. It never falls back to a picker — it cannot even open one |

The default a `Resolve` returns is never destructive. That is a rule, not an
observation: `clean --yes` leaves children orphaned unless `--reparent-children`,
`sync --yes` does not push.

**`--force` never travels this path.** It is not an answer, it is a field of the
`Request` — the safety axis. Two consequences worth keeping in mind:

- `--force` alone does **not** imply `--yes`. The session still runs and still asks to
  confirm; the refusals are simply already lifted.
- `--yes` alone does **not** lift a refusal. `clean --yes` on a dirty worktree fails,
  naming `--force`: see `resolveDelete` in `internal/flow/clean/steps.go`, which runs
  the safety check *while* answering the step.

The command's only job on this axis is choosing the Prompter, in one line:

```go
interactive := rules.IsHumanFormat(format) && !yes && term.IsTerminal(int(os.Stdin.Fd()))
// ...
Prompter: flowPrompter(flowPrompterParams{Interactive: interactive}),
```

## Hook output, from the service writer to the dashboard panel

`hooks.RunHooks` sets `cmd.Stdout = output` on each hook, so hook stdout is already
incremental at the source. What the flow layer adds is a way for a surface to receive
it as *events* rather than as a stream.

```mermaid
flowchart LR
  F["flow: HookPhase, Run(sink) calls RunCreateHooks with Output = sink"] --> P["Presenter.HookPhase"]
  P -->|CLI| S1["sink = cmd.ErrOrStderr — bytes straight through"]
  P -->|dashboard| S2["sink = flow.LineWriter"]
  S1 --> Term["the terminal, output unchanged"]
  S2 --> M["one OutputLineMsg per line"]
  M --> Panel["the dashboard's bottom output panel"]
```

The flow never writes: it asks the Presenter for a sink and hands that sink to the
service. `flow.LineWriter` is an `io.Writer` that buffers the current fragment and
emits on each `\n`, with a `Flush` for the trailing partial line. It lives in `flow/`
because it depends on nothing but the stdlib.

**The concurrency point.** `RunHooks` is synchronous and writes from the calling
goroutine. So the dashboard runs the flow in a goroutine, and `LineWriter.Emit` posts a
`tea.Msg` on the dashboard's channel — it never mutates a model. This is the whole
reason `presenter.HookPhase` reads the way it does:

```go
func (p presenter) HookPhase(params flow.HookPhaseParams) error {
	p.line(params.Title)
	sink := &flow.LineWriter{Emit: p.line} // p.line sends a tea.Msg
	err := params.Run(sink)
	sink.Flush()
	return err
}
```

`LineWriter` is not concurrency-safe, and does not need to be: one hook run, one
writer, one goroutine.

What stays buffered on purpose is each hook's **stderr** — it is only printed when the
hook fails, under the `✗` line. Streaming it would change how CLI output interleaves.

## Testing a flow

A flow is tested without a terminal, by handing it the two doubles in
`internal/testutil/flowtest`:

```go
prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{
	create.KeyBranch: "feat/x",
	create.KeySource: "main",
}}
recorder := &flowtest.Recorder{}
```

- **`ScriptedPrompter`** walks the session the way a real host does — honoring `Skip`,
  `Build` and `Load` — and answers each step from the script. It records `Asked` (with
  `AskedKeys()` for a one-line assertion) and the `StepContent` each step produced, so
  a test can assert on *what the user would have seen*, not only on the outcome. A step
  with nothing scripted is an error, so a new question cannot slip into a flow
  unnoticed.
- **`Recorder`** implements `flow.Presenter`, collecting `Stages`, `Hooks`, `Notices`
  and `Statuses`. It runs `Work()` and `Run(sink)` for real, so the service still gets
  called.

The typed conclusion (`Created`, `Cleaned`) is not part of `Recorder` — a test that
needs it embeds the recorder and adds the one method:

```go
type recorder struct {
	*flowtest.Recorder
	outcome create.Outcome
}

func (r *recorder) Created(o create.Outcome) error { r.outcome = o; return nil }
```

For the unattended path, `flow.Unattended{}` **is** the test double: passing it
directly is how the resolution taxonomy is tested (`internal/flow/unattended_test.go`).

**Characterization tests.** Before `create` and `clean` were migrated, their observable
CLI behavior was pinned by tests written against the *old* code:
`internal/tui/newwt/create_flow_test.go`, `internal/commands/wt/create_wizard_test.go`,
`create_noninteractive_test.go` and `integration_test.go` (the `--yes` / `--force` axes,
the JSON reparent default, idempotence on an absent worktree). `prune` got the same
treatment in `internal/commands/wt/prune_test.go`, which needed two new fixtures to
reach its core at all: `internal/testutil/ghtest` scripts the GitHub CLI through `PATH`,
and `gittest.AddOrigin` gives branches a real upstream. They exist to be run
unchanged after the refactor. Keep them that way: they are the only thing that proves a
flow that moved packages still reads the same to a user. When you migrate a command,
write its characterization tests first, and do not "fix" one to make a refactor pass.

## Two decisions worth not re-opening

### A non-mutating mode is a business input (`prune --dry-run`)

`--dry-run` looks like an output mode, and the first instinct is to keep it out of the
`Request` on the grounds that a request carries business inputs, not presentation. That
reading is wrong: `--dry-run` does not change *how the run reads*, it changes *what the
run does* — nothing. It belongs with `--force` on the input side, the way `terraform
plan` sits beside `apply`.

So `prune.Request.DryRun` exists, and `Run` returns an `Outcome` carrying the plan
before it asks a single question and before it touches anything. The alternative — a
second exported `Plan()` the runner calls instead of `Run()` — would have kept a plan
computation path in `commands/` and forced the `gh` advisory to be emitted from two
places.

One trap comes with it, and it is why `rules.PruneClassifyForce` takes `DryRun` as an
input rather than being a `||` in the flow. A surface may perfectly well install an
interactive Prompter *and* set `DryRun`. Classifying with force because "someone could
uncheck" would then make a preview list worktrees a real run would have skipped. The
rule states the three-term condition once, and tests it.

### The three reparent service functions stay three

`worktree.ReparentBatch` (for `wtm reparent`), `worktree.ApplyReparentChildren` (for
`clean`) and `worktree.ApplyReparents` (for `prune`) all rewrite a worktree's recorded
parent. `prune` being the last of the three callers to migrate, the question of folding
them into one was raised deliberately here — and answered: **no.**

- There is **no duplicated logic to remove.** All three funnel into `setSourceBranch`,
  which is already the single chokepoint for the write.
- Their contracts genuinely differ. Only `ReparentBatch` validates acyclicity, because
  only it moves worktrees onto a parent the user chose. The other two reattach children
  to a grandparent, which cannot close a cycle by construction — giving them the check
  would be dead validation.
- Merging them yields one function with behaviour flags (`Validate bool`,
  `Grandparent bool`) that is harder to read than the three signatures it replaces, and
  hides which caller is allowed to skip which check.

What is broad is the *exported surface*, not the logic. Three names for one idea is a
fair price for three honest contracts. Do not consolidate them without a new reason.

## Known gaps

Deliberately open, tracked, and not to be fixed opportunistically:

| Ticket | Gap |
| -- | -- |
| LUC-179 | `clean --force` alone without a TTY skips the safety check — pre-existing, made visible by this design |
| LUC-180 | `flow.Context` duplicates `shared.ConfigResult` (the latter imports cobra, so it cannot be reused as is) |
| LUC-182 | `extract` is not migrated; create's step declaration therefore exists twice |
| LUC-183 | `flow.Step` carries kind-specific fields (`Branches`, `Pinned`, `Refresh`, `Validate`/`ValidateSet`) on every kind. It also means a `StepBranchSelect` reads its candidates from `Step.Branches`, before any answer exists, so it cannot narrow them from an earlier step — `reparent` narrows from what its request already names instead |
| LUC-184 | Locked worktrees are only taken into account by `relocate`, so "locked" is not among clean's blockers |
| LUC-188 | `busyReason("")` only sees blocking runs, so a `ModeBackground` run holding a worktree does not stop a `ModeBlocking` run with no target (the batch reparent, `prune`) from acting on it |
| — | When every prune match is skipped, the run reports an empty result instead of the skips that explain it. Pinned by `TestPruneAllUnsafeReportsNothing` so a refactor cannot change it silently; fixing it is its own decision |
