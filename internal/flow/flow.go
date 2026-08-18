// Package flow carries the déroulé of the wtm commands, independently of the
// surface that runs them.
package flow

import (
	"fmt"
	"io"

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
)

type Option struct {
	Label     string
	Value     string
	Separator bool
	Danger    bool
}

type StepContent struct {
	Title       string
	Description string
	Options     []Option
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
	Skip     func(Answers) (skip bool, reason string)
	Build    func(Answers) (StepContent, error)

	Load           func(Answers) (StepContent, error)
	LoadingMessage string

	// Resolve answers the step with no interaction: an Answer is a safe default, an
	// error refuses the run and must name the flag. Nil means "interactive only".
	Resolve func(Answers) (Answer, error)

	Summarize func(Answer) string
	Flag      string
}

type Session struct {
	ErrLabel string
	Steps    []Step
	Presets  Answers
}

type Answer struct {
	Value      string
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
}

type Presenter interface {
	Stage(StageParams) error
	HookPhase(HookPhaseParams) error
	// Notice concludes the run; Status is one line inside an ongoing phase.
	Notice(Notice)
	Status(Notice)
}

var AbortedNotice = Notice{Kind: NoticeMessage, Text: domain.AbortedMessage}

func requiredErr(step Step) error {
	if step.Flag == "" {
		return fmt.Errorf(domain.FlowStepRequiredFmt, step.Label)
	}
	return fmt.Errorf(domain.FlowStepRequiredFlagFmt, step.Label, step.Flag)
}
