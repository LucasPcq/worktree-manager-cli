package flow

// Unattended is the Prompter of a run that cannot ask anything: --yes, no
// terminal, or a machine output format. It carries the whole bypass taxonomy, once
// and for every command, so no command reimplements it:
//
//  1. the value is already known (a flag, a positional argument) → it is used;
//  2. the step is irrelevant given the earlier answers → it is skipped;
//  3. the step has a safe default → Resolve supplies it, never destructively;
//  4. the step is a required selection with no safe default → the run is refused
//     with an error naming the missing flag.
//
// A picker is never a fallback: Unattended cannot show one.
type Unattended struct{}

// Ask resolves every step without interaction, in declaration order, so a later
// step's Resolve sees the answers of the earlier ones.
func (Unattended) Ask(session Session) (Answers, error) {
	answers := session.Presets
	for _, step := range session.Steps {
		if _, ok := answers.Get(step.Key); ok {
			continue
		}
		if step.Skip != nil {
			if skip, reason := step.Skip(answers); skip {
				answers = answers.With(step.Key, Answer{Skipped: true, SkipReason: reason})
				continue
			}
		}
		if step.Resolve == nil {
			return Answers{}, requiredErr(step)
		}
		answer, err := step.Resolve(answers)
		if err != nil {
			return Answers{}, err
		}
		answers = answers.With(step.Key, answer)
	}
	return answers, nil
}

// Confirm declines every standalone confirmation: there is nobody to ask, and a
// post-execution recovery must never be assumed.
func (Unattended) Confirm(ConfirmParams) (bool, error) { return false, nil }

// Interactive reports that no decision may be offered.
func (Unattended) Interactive() bool { return false }
