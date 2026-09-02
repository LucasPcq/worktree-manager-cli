package up

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/rules"
)

const KeyConcurrency = "run.concurrency"

// The four answers: what to do, and whether to be asked again. The "always"
// pair writes the choice to run.toml, which is the whole point of asking here
// rather than at every start.
// The plain answers are the domain values themselves, so what Resolve produces
// with no interaction and what the picker produces read back the same way.
const (
	answerParallel        = string(domain.ConcurrencyParallel)
	answerExclusive       = string(domain.ConcurrencyExclusive)
	answerParallelAlways  = answerParallel + alwaysSuffix
	answerExclusiveAlways = answerExclusive + alwaysSuffix
	alwaysSuffix          = "-always"
)

func (f *upFlow) session() flow.Session {
	return flow.Session{
		ErrLabel: domain.CmdUp,
		Presets:  f.presets(),
		Steps: []flow.Step{
			target.WorktreeStep(target.WorktreeParams{
				ProjectDir: f.ctx.ProjectDir,
				Current:    f.request.Cwd,
				Running:    f.running,
			}),
			target.ProfileStep(target.ProfileParams{
				Profiles: f.request.Config.Profiles,
				Default:  f.defaultProfile(),
			}),
			f.concurrencyStep(),
		},
	}
}

// presets carry what the flags already answered: the step is not asked, but it
// is still read back, so a flag never makes a line vanish from the recap.
func (f *upFlow) presets() flow.Answers {
	values := map[string]string{target.KeyProfile: f.request.Profile}
	if f.named != nil {
		values[target.KeyWorktree] = f.named.Dir
	}
	return flow.NewAnswers(values)
}

// concurrencyStep is asked at most once per project. It is skipped when nothing
// runs elsewhere — there is nothing to stop — and when run.toml already holds
// the answer.
func (f *upFlow) concurrencyStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepSelect,
		Key:   KeyConcurrency,
		Label: domain.RunConcurrencyStepName,
		Skip: func(answers flow.Answers) (bool, string) {
			decision := f.decideConcurrency(answers)
			if decision.Ask {
				return false, ""
			}
			if !f.othersRunning(answers) {
				return true, domain.RunConcurrencySkipAlone
			}
			return true, domain.RunConcurrencySkipSettled
		},
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{
				Title:       domain.RunConcurrencyTitle,
				Description: fmt.Sprintf(domain.RunConcurrencyDescFmt, f.othersSummary(answers)),
				Options: []flow.Option{
					{Label: domain.RunConcurrencyParallel, Value: answerParallel},
					{Label: alwaysLabel(domain.RunConcurrencyParallel), Value: answerParallelAlways},
					{Separator: true},
					{Label: domain.RunConcurrencyExclusive, Value: answerExclusive},
					{Label: alwaysLabel(domain.RunConcurrencyExclusive), Value: answerExclusiveAlways},
				},
			}, nil
		},
		// Leaving the others alone is the answer that stops nothing, which is what
		// a safe default means here.
		Resolve: func(answers flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: string(f.decideConcurrency(answers).Value)}, nil
		},
		Summarize: func(answer flow.Answer) string { return string(concurrencyOf(answer.Value)) },
	}
}

func alwaysLabel(label string) string {
	return fmt.Sprintf(domain.RunConcurrencyAlwaysFmt, label)
}

// concurrencyOf reads any of the four answers, remembered or not.
func concurrencyOf(answer string) domain.Concurrency {
	if strings.TrimSuffix(answer, alwaysSuffix) == answerExclusive {
		return domain.ConcurrencyExclusive
	}
	return domain.ConcurrencyParallel
}

func remembers(answer string) bool { return strings.HasSuffix(answer, alwaysSuffix) }

func (f *upFlow) decideConcurrency(answers flow.Answers) rules.ConcurrencyDecision {
	return rules.DecideConcurrency(rules.ConcurrencyParams{
		Exclusive:     f.request.Exclusive,
		Parallel:      f.request.Parallel,
		Config:        f.request.Config.Concurrency,
		OthersRunning: f.othersRunning(answers),
	})
}

// othersRunning is measured against the worktree the run targets, never against
// the current directory: `run up X` must not offer to stop X's own jobs.
func (f *upFlow) othersRunning(answers flow.Answers) bool {
	return len(f.otherWorktrees(answers)) > 0
}

func (f *upFlow) otherWorktrees(answers flow.Answers) map[string][]string {
	targetDir := f.workDir(answers)
	others := make(map[string][]string)
	for _, job := range f.jobs {
		if !rules.IsJobUp(job.Status) || job.WorkDir == targetDir {
			continue
		}
		others[job.WorkDir] = append(others[job.WorkDir], job.Name)
	}
	return others
}

// othersSummary names the worktrees and their jobs, so the question says what
// it is about rather than that something is running somewhere.
func (f *upFlow) othersSummary(answers flow.Answers) string {
	others := f.otherWorktrees(answers)
	dirs := make([]string, 0, len(others))
	for dir := range others {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	lines := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		lines = append(lines, fmt.Sprintf("%s (%s)", filepath.Base(dir), strings.Join(others[dir], domain.RunURLListSep)))
	}
	return strings.Join(lines, domain.RunURLListSep)
}

func (f *upFlow) defaultProfile() string {
	profile, ok := rules.DefaultProfile(f.request.Config)
	if !ok {
		return ""
	}
	return profile.Name
}
