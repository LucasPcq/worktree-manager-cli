package flow

import (
	"errors"
	"strings"
	"testing"
)

var errNeedsFlag = errors.New("pass --thing")

func TestUnattendedUsesThePresetValue(t *testing.T) {
	session := Session{
		Presets: NewAnswers(map[string]string{"a": "from-flag"}),
		Steps: []Step{{
			Key: "a",
			Resolve: func(Answers) (Answer, error) {
				t.Fatal("a step whose value is already known must not be resolved again")
				return Answer{}, nil
			},
		}},
	}

	answers, err := (Unattended{}).Ask(session)
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if got := answers.Value("a"); got != "from-flag" {
		t.Errorf("value = %q, want the preset", got)
	}
	if answers.Answered("a") {
		t.Error("a preset is not an answer a human gave")
	}
}

func TestUnattendedFallsBackToTheSafeDefault(t *testing.T) {
	session := Session{Steps: []Step{{
		Key:     "a",
		Resolve: func(Answers) (Answer, error) { return Answer{Value: "safe"}, nil },
	}}}

	answers, err := (Unattended{}).Ask(session)
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if got := answers.Value("a"); got != "safe" {
		t.Errorf("value = %q, want the safe default", got)
	}
}

func TestUnattendedRefusesARequiredSelection(t *testing.T) {
	session := Session{Steps: []Step{{
		Key:     "a",
		Resolve: func(Answers) (Answer, error) { return Answer{}, errNeedsFlag },
	}}}

	if _, err := (Unattended{}).Ask(session); !errors.Is(err, errNeedsFlag) {
		t.Fatalf("err = %v, want the step's own refusal", err)
	}
}

// The backstop: a step with no fallback refuses the run rather than resolving to
// nothing or opening a picker.
func TestUnattendedRefusesAStepItCannotResolve(t *testing.T) {
	_, err := (Unattended{}).Ask(Session{Steps: []Step{{Key: "a", Label: "Thing", Flag: "thing"}}})
	if err == nil {
		t.Fatal("expected a refusal")
	}
	if !strings.Contains(err.Error(), "--thing") {
		t.Errorf("error %q should name the flag", err)
	}
}

func TestUnattendedSkipsAnIrrelevantStep(t *testing.T) {
	session := Session{Steps: []Step{{
		Key:  "a",
		Skip: func(Answers) (bool, string) { return true, "nothing to do" },
		Resolve: func(Answers) (Answer, error) {
			t.Fatal("a skipped step must not be resolved")
			return Answer{}, nil
		},
	}}}

	answers, err := (Unattended{}).Ask(session)
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	answer, _ := answers.Get("a")
	if !answer.Skipped || answer.SkipReason != "nothing to do" {
		t.Errorf("answer = %+v, want it skipped with a reason", answer)
	}
	if answer.Value != "" {
		t.Errorf("value = %q, want empty for a skipped step", answer.Value)
	}
}

// A later step's fallback reads the earlier answers, which is how create's source
// default depends on the branch it was given.
func TestUnattendedResolvesInOrder(t *testing.T) {
	session := Session{
		Presets: NewAnswers(map[string]string{"first": "one"}),
		Steps: []Step{
			{Key: "first", Resolve: func(Answers) (Answer, error) { return Answer{Value: "unused"}, nil }},
			{Key: "second", Resolve: func(a Answers) (Answer, error) {
				return Answer{Value: a.Value("first") + "-two"}, nil
			}},
		},
	}

	answers, err := (Unattended{}).Ask(session)
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if got := answers.Value("second"); got != "one-two" {
		t.Errorf("value = %q, want it derived from the earlier answer", got)
	}
}

func TestUnattendedNeverConfirmsAndIsNotInteractive(t *testing.T) {
	confirmed, err := (Unattended{}).Confirm(ConfirmParams{DefaultYes: true})
	if err != nil || confirmed {
		t.Errorf("Confirm = (%v, %v), want (false, nil)", confirmed, err)
	}
	if (Unattended{}).Interactive() {
		t.Error("Interactive must be false")
	}
}

func TestAnswersValuesReadsASetAnswer(t *testing.T) {
	answers := Answers{}.WithValues("a", []string{"one", "two"})

	got := answers.Values("a")
	if len(got) != 2 || got[0] != "one" || got[1] != "two" {
		t.Errorf("values = %v, want the set as answered", got)
	}
}

// A caller wanting a list must not have to know which kind produced the answer.
func TestAnswersValuesReadsASingleAnswerAsASetOfOne(t *testing.T) {
	answers := NewAnswers(map[string]string{"a": "only"})

	got := answers.Values("a")
	if len(got) != 1 || got[0] != "only" {
		t.Errorf("values = %v, want a set of one", got)
	}
	if unanswered := answers.Values("b"); unanswered != nil {
		t.Errorf("values = %v, want nil for an unanswered key", unanswered)
	}
}

// An empty set is unanswered, as an empty string is for a single-valued step.
func TestWithValuesIgnoresAnEmptySet(t *testing.T) {
	answers := Answers{}.WithValues("a", nil)

	if _, known := answers.Get("a"); known {
		t.Error("an empty set must not count as an answer")
	}
}

func TestUnattendedRefusesAMultiSelectWithNoResolve(t *testing.T) {
	session := Session{Steps: []Step{{
		Kind: StepMultiSelect, Key: "a", Label: "Worktrees",
	}}}

	_, err := (Unattended{}).Ask(session)
	if err == nil {
		t.Fatal("a set nobody can be asked for must refuse the run")
	}
	if !strings.Contains(err.Error(), "Worktrees") {
		t.Errorf("err = %v, want it to name the step", err)
	}
}
