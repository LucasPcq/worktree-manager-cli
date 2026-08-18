// Package flow carries the business déroulé of the wtm commands: the sequence of
// decisions, service calls and phases a command performs, independently of the
// surface that runs it (the CLI wizard today, a dashboard tomorrow).
//
// Imports are restricted to internal/service, internal/rules, internal/domain and
// the stdlib — never cobra, bubbletea or lipgloss, and never internal/output,
// internal/tui, internal/config or internal/commands. A surface plugs in through
// three seams: Prompter (who answers), Presenter (what is shown) and a
// per-command Request (what is already known).
package flow

import (
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
)

// Context is the resolved environment of a run. flow cannot import
// internal/config (it reads cobra flags and the environment), so the surface
// loads the config and hands the result over.
type Context struct {
	ProjectDir string
	StateDir   string
	Config     domain.Config
}

// StepKind classifies what a step asks for. Only the kinds create and clean need
// exist today; StepMultiSelect and StepConfirm join when extract and sync migrate
// (a surface must reject a kind it cannot render rather than guess).
type StepKind int

const (
	// StepText asks for a free-form value (a branch name).
	StepText StepKind = iota
	// StepSelect asks for one value among Options.
	StepSelect
	// StepBranchSelect asks for one branch among Branches. It is distinct from
	// StepSelect because a branch row carries its divergence from origin, which each
	// surface renders its own way, and because the candidate list can be refreshed.
	StepBranchSelect
	// StepRecap is the final synthesis: it recaps the answers and is the single
	// point where the operation is confirmed or cancelled.
	StepRecap
)

// Option is one row of a StepSelect. Purely descriptive — each surface decides how
// to render it.
type Option struct {
	Label string
	Value string
	// Separator marks a non-selectable spacing row.
	Separator bool
	// Danger marks a destructive choice.
	Danger bool
}

// StepContent is the part of a step that may depend on the earlier answers.
type StepContent struct {
	Title       string
	Description string
	Options     []Option
}

// Step declares one question. A step is data: it never renders and never blocks.
type Step struct {
	Kind StepKind
	// Key identifies the answer in Answers — the contract between a flow and its
	// own readers. Stable across surfaces.
	Key string
	// Label names the step in a host's progress indicator (the CLI breadcrumb).
	Label string
	// Title heads the question itself; empty when the description carries it.
	Title       string
	Description string
	Options     []Option
	// Branches are the candidates of a StepBranchSelect, and Pinned is the one
	// pinned first as the suggested default.
	Branches []domain.BranchCandidate
	Pinned   string
	// Refresh re-fetches the candidates of a StepBranchSelect (it hits the network,
	// so a host runs it off its render path). Nil disables refreshing.
	Refresh func() []domain.BranchCandidate
	// Validate rejects a StepText value.
	Validate func(string) error
	// Skip reports, from the earlier answers, that the step is irrelevant, and why
	// ("source already up to date"). The reason is surfaced by the host.
	Skip func(Answers) (skip bool, reason string)
	// Build re-derives the content from the earlier answers each time the step is
	// entered. Use it when a title, description or option list depends on an
	// earlier answer. Its error refuses the run: a host builds the content of the
	// steps it is about to show before showing any of them.
	Build func(Answers) (StepContent, error)
	// Load is Build's slow sibling: deriving the content needs I/O (a git status, a
	// network check). A host runs it off its render path behind a loading state.
	Load func(Answers) (StepContent, error)
	// LoadingMessage is shown while Load runs.
	LoadingMessage string
	// Resolve answers the step with no interaction at all, from the earlier
	// answers. It is the whole of the bypass taxonomy: returning an Answer is a
	// decision with a safe default, returning an error refuses the run — and that
	// error must name the flag that would have supplied the value. A nil Resolve
	// means the step can only be answered interactively.
	Resolve func(Answers) (Answer, error)
	// Summarize labels the answer once the step is done; nil lets the host use the
	// answer's own value.
	Summarize func(Answer) string
	// Flag names the CLI flag supplying this step's value, for the refusal message
	// of a step with no Resolve.
	Flag string
}

// Session is one uninterrupted question-and-recap sequence. A host renders it as a
// single unit (the CLI: one wizard, with a breadcrumb and back navigation), so
// cancelling anywhere cancels the whole sequence.
type Session struct {
	// ErrLabel prefixes an infrastructure failure of the host ("wizard: …").
	ErrLabel string
	Steps    []Step
	// Presets answers steps whose value is already known (flags, positional args).
	// A preset step is not asked, but a flow still reads it back from Answers — so a
	// flag never makes a line vanish from the recap.
	Presets Answers
}

// Answer is one step's outcome.
type Answer struct {
	Value  string
	Values []string
	// Skipped reports that the step was irrelevant, with the reason Skip gave.
	Skipped    bool
	SkipReason string
	// Asked reports that a human actually answered; false for a preset, a Resolve
	// fallback, or a skipped step.
	Asked bool
}

// Answers maps step keys to answers. It is immutable: With returns a new value.
type Answers struct {
	byKey map[string]Answer
}

// NewAnswers builds an Answers from step-key/value pairs, for the presets a
// surface already holds. An empty value is not a preset: it means "unanswered".
func NewAnswers(values map[string]string) Answers {
	byKey := make(map[string]Answer, len(values))
	for key, value := range values {
		if value == "" {
			continue
		}
		byKey[key] = Answer{Value: value}
	}
	return Answers{byKey: byKey}
}

// With returns a copy carrying one more answer.
func (a Answers) With(key string, answer Answer) Answers {
	byKey := make(map[string]Answer, len(a.byKey)+1)
	for k, v := range a.byKey {
		byKey[k] = v
	}
	byKey[key] = answer
	return Answers{byKey: byKey}
}

// Get returns the answer for key, and whether it exists.
func (a Answers) Get(key string) (Answer, bool) {
	answer, ok := a.byKey[key]
	return answer, ok
}

// Value returns the answer's value, or "" when the step was never answered.
func (a Answers) Value(key string) string {
	return a.byKey[key].Value
}

// Values returns a multi-valued answer, or nil.
func (a Answers) Values(key string) []string {
	return a.byKey[key].Values
}

// Answered reports whether a human answered the step (not a preset, a fallback or
// a skip).
func (a Answers) Answered(key string) bool {
	answer, ok := a.byKey[key]
	return ok && answer.Asked && !answer.Skipped
}

// ConfirmParams describes a standalone confirmation, outside any session.
type ConfirmParams struct {
	Title       string
	Description string
	Warning     string
	DefaultYes  bool
}

// Prompter answers the questions a flow asks. Three implementations: the CLI
// wizard (internal/tui/flowui), Unattended (below) and the dashboard.
type Prompter interface {
	// Ask runs a whole Session and returns every answer, keyed by Step.Key.
	// Returns domain.ErrUserAborted when the run is cancelled.
	Ask(Session) (Answers, error)
	// Confirm asks a single question outside any session — a decision that can only
	// exist after an execution (a fast-forward that failed, a removal needing
	// privileges). Returns domain.ErrUserAborted when cancelled.
	Confirm(ConfirmParams) (bool, error)
	// Interactive reports whether a decision may be offered at all. A flow reads it
	// for two things only: not offering a post-execution recovery nobody can answer,
	// and feeding a pure rule that takes it as an input (rules.DecidePush).
	Interactive() bool
}

// StageParams holds inputs for Presenter.Stage.
type StageParams struct {
	Message string
	Work    func() error
}

// HookPhaseParams holds inputs for Presenter.HookPhase. Run receives the writer
// the hooks stream into: the CLI hands over its stderr, so hook output keeps
// flowing line by line as it is produced; a TUI hands over a LineWriter.
type HookPhaseParams struct {
	Title string
	Run   func(io.Writer) error
}

// NoticeKind selects how a notice reads.
type NoticeKind int

const (
	// NoticeMessage is a neutral statement.
	NoticeMessage NoticeKind = iota
	// NoticeWarning is a refusal or a caveat.
	NoticeWarning
	// NoticeSuccess is a completed side effect.
	NoticeSuccess
)

// Notice is one line of prose a flow decided on and a surface renders.
type Notice struct {
	Kind NoticeKind
	Text string
}

// Presenter renders the phases of a flow. The flow never draws, never frames and
// never chooses a stream.
type Presenter interface {
	// Stage runs one unit of work under a progress indicator.
	Stage(StageParams) error
	// HookPhase opens a titled hook section and runs the hooks against the writer
	// it provides.
	HookPhase(HookPhaseParams) error
	// Notice reports a self-contained message that concludes the run: an idempotent
	// no-op, a refusal, an abort.
	Notice(Notice)
	// Status reports one line inside an ongoing phase: a service that was stopped,
	// a failure being recovered from.
	Status(Notice)
}

// AbortedNotice is the message every command shows when the user cancels.
var AbortedNotice = Notice{Kind: NoticeMessage, Text: domain.AbortedMessage}

// requiredErr refuses a step that has no Resolve and could not be answered — case
// 2 of the bypass taxonomy: a required selection with no safe default. A flow
// normally gives such a step a Resolve returning its own worded refusal; this is
// the backstop that keeps "fall back to a picker" from ever being an option.
func requiredErr(step Step) error {
	if step.Flag == "" {
		return fmt.Errorf("%s is required and cannot be asked in this mode", step.Label)
	}
	return fmt.Errorf("%s is required and cannot be asked in this mode: pass --%s", step.Label, step.Flag)
}
