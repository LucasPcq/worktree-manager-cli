// Package flow carries the run of each wtm command, independently of the
// surface that drives it.
package flow

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
)

type Context struct {
	ProjectDir string
	StateDir   string
	Config     domain.Config
}

type StepKind int

const (
	StepText StepKind = iota
	StepSelect
	StepBranchSelect
	StepRecap
	// StepMultiSelect asks for a set rather than a value; its answer is carried by
	// Answer.Values.
	StepMultiSelect
)

type Option struct {
	Label     string
	Value     string
	Separator bool
	Danger    bool
	// Selected pre-checks the option in a StepMultiSelect, so a step can offer a
	// set it already narrowed rather than an empty one.
	Selected bool
	// Tag is a short status word shown before the label, coloured by Tone.
	Tag  string
	Tone domain.Tone
}

type StepContent struct {
	Title       string
	Description string
	Options     []Option
	// Blockers are the refusals the step folds into its Description, named one by
	// one for a surface that can have each of them lifted separately.
	Blockers []Blocker
	// ExcludeBranches drops candidates from a StepBranchSelect by name. A step
	// narrows this way rather than by handing over a list, so the background
	// refresh stays authoritative on what exists — the exclusion is applied on top
	// of whatever it last returned.
	ExcludeBranches []string
}

// Blocker is one safety refusal standing in the way of the step's dangerous
// option, stated on its own so nothing is ever lifted implicitly.
type Blocker struct {
	Key   string
	Label string
}

type Step struct {
	Kind        StepKind
	Key         string
	Label       string
	Title       string
	Description string
	Options     []Option

	Branches []domain.BranchCandidate
	Pinned   string
	Refresh  func() []domain.BranchCandidate

	Validate func(value string) error
	// ValidateSet is Validate for a StepMultiSelect step.
	ValidateSet func(values []string) error
	Skip        func(Answers) (skip bool, reason string)
	Build       func(Answers) (StepContent, error)

	Load           func(Answers) (StepContent, error)
	LoadingMessage string

	// Resolve answers the step with no interaction: an Answer is a safe default, an
	// error refuses the run and must name the flag. Nil means "interactive only".
	Resolve func(Answers) (Answer, error)

	Summarize func(Answer) string
	Flag      string
}

// Mode is how long a flow holds the surface that runs it. A background flow gives
// the surface back and locks its target instead, so nothing else acts on the
// worktree it is still working on.
type Mode int

const (
	ModeBlocking Mode = iota
	ModeBackground
)

// Operation is what a surface needs to schedule a flow without knowing what it
// does: how it holds the surface, and which answer names the worktree it holds.
type Operation struct {
	Kind      string
	Mode      Mode
	TargetKey string
}

type Session struct {
	ErrLabel string
	Steps    []Step
	Presets  Answers
}

type Answer struct {
	Value string
	// Values is the answer of a StepMultiSelect step; every other kind leaves it
	// nil and answers with Value.
	Values     []string
	Skipped    bool
	SkipReason string
	Asked      bool
}

type Answers struct {
	byKey map[string]Answer
}

// NewAnswers builds the answers a surface already holds. An empty value means
// unanswered, not answered-with-nothing.
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

func (a Answers) With(key string, answer Answer) Answers {
	byKey := make(map[string]Answer, len(a.byKey)+1)
	for k, v := range a.byKey {
		byKey[k] = v
	}
	byKey[key] = answer
	return Answers{byKey: byKey}
}

func (a Answers) Get(key string) (Answer, bool) {
	answer, ok := a.byKey[key]
	return answer, ok
}

func (a Answers) Value(key string) string { return a.byKey[key].Value }

// Values reads a set answer. A single-valued answer reads back as a set of one,
// so a caller that wants a list never has to know which kind produced it.
func (a Answers) Values(key string) []string {
	answer := a.byKey[key]
	if len(answer.Values) > 0 {
		return answer.Values
	}
	if answer.Value == "" {
		return nil
	}
	return []string{answer.Value}
}

// WithValues answers a StepMultiSelect step. An empty set means unanswered, as
// an empty string does for a single-valued one.
func (a Answers) WithValues(key string, values []string) Answers {
	if len(values) == 0 {
		return a
	}
	return a.With(key, Answer{Values: values})
}

// Answered excludes a preset, a Resolve fallback and a skip.
func (a Answers) Answered(key string) bool {
	answer, ok := a.byKey[key]
	return ok && answer.Asked && !answer.Skipped
}

type ConfirmParams struct {
	Title       string
	Description string
	Warning     string
	DefaultYes  bool
	// YesLabel and NoLabel name the two outcomes instead of answering yes or no.
	// A decision whose consequences differ (keeping local vs force-pushing) is
	// asked this way; empty labels keep the plain confirmation both surfaces
	// already render.
	YesLabel string
	NoLabel  string
}

type Prompter interface {
	Ask(Session) (Answers, error)
	Confirm(ConfirmParams) (bool, error)
	Interactive() bool
}

type StageParams struct {
	Message string
	Work    func() error
}

type HookPhaseParams struct {
	Title string
	Run   func(sink io.Writer) error
}

type NoticeKind int

const (
	NoticeMessage NoticeKind = iota
	NoticeWarning
	NoticeSuccess
)

type Notice struct {
	Kind NoticeKind
	Text string
	// Lines turn the notice into a titled block, Text being its title. Empty
	// keeps it the single line every other caller emits.
	Lines []string
}

type Presenter interface {
	Stage(StageParams) error
	HookPhase(HookPhaseParams) error
	// Notice concludes the run; Status is one line inside an ongoing phase.
	Notice(Notice)
	Status(Notice)
}

var AbortedNotice = Notice{Kind: NoticeMessage, Text: domain.AbortedMessage}

// IsAbort identifies the notice a flow emits when the user backed out. Notice
// carries a slice, so it cannot be compared with ==.
func (n Notice) IsAbort() bool {
	return n.Kind == AbortedNotice.Kind && n.Text == AbortedNotice.Text
}

func requiredErr(step Step) error {
	if step.Flag == "" {
		return fmt.Errorf(domain.FlowStepRequiredFmt, step.Label)
	}
	return fmt.Errorf(domain.FlowStepRequiredFlagFmt, step.Label, step.Flag)
}

// ResolveSymlinks canonicalizes a path when it still exists, and hands it back
// untouched when it does not. A flow deciding whether the cwd sits inside what it
// is about to remove has to compare canonical paths, and has to do it before the
// removal.
func ResolveSymlinks(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

// KeepBranches drops the excluded names from a candidate list. Both surfaces
// rendering a StepBranchSelect apply the step's exclusions this way, so the two
// cannot disagree on what a narrowed picker shows.
func KeepBranches(candidates []domain.BranchCandidate, exclude []string) []domain.BranchCandidate {
	if len(exclude) == 0 {
		return candidates
	}
	dropped := make(map[string]bool, len(exclude))
	for _, name := range exclude {
		dropped[name] = true
	}
	kept := make([]domain.BranchCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if dropped[candidate.Name] {
			continue
		}
		kept = append(kept, candidate)
	}
	return kept
}
