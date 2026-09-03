package target

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
)

type WorktreeParams struct {
	ProjectDir string
	// Current is the worktree the command was launched from, as git spells it.
	// It is two things at once: the option the cursor opens on, and the answer
	// when nobody can be asked — `run` is the one module whose subject has a safe
	// default, because standing in a worktree is what designates it.
	Current string
	// Running counts the jobs each worktree has up, for a reader deciding which
	// one to act on. Empty simply leaves the badges off.
	Running map[string]int
}

// WorktreeStep asks which worktree the command acts on. It is skipped when the
// repository holds a single one — a list of one asks nothing — and answers with
// the current worktree whenever it is not asked.
func WorktreeStep(params WorktreeParams) flow.Step {
	list := &worktreeList{projectDir: params.ProjectDir}
	// Spelled the way git spells it, whatever the caller passed: the listing is,
	// and on macOS a /var that git calls /private/var would otherwise match no
	// row — losing the cursor and the "current" badge without a word.
	params.Current = Root(params.Current)

	return flow.Step{
		Kind:  flow.StepSelect,
		Key:   KeyWorktree,
		Label: domain.RunWorktreeStepName,
		Skip: func(flow.Answers) (bool, string) {
			worktrees, err := list.get()
			if err != nil {
				return true, domain.RunWorktreeUnreadable
			}
			if len(worktrees) <= 1 {
				return true, domain.RunWorktreeOnlyOne
			}
			return false, ""
		},
		Build: func(flow.Answers) (flow.StepContent, error) {
			worktrees, err := list.get()
			if err != nil {
				return flow.StepContent{}, fmt.Errorf("list worktrees: %w", err)
			}
			return flow.StepContent{
				Title:       domain.RunWorktreePickerTitle,
				Description: domain.RunWorktreePickerDesc,
				Options:     worktreeOptions(worktrees, params),
				Start:       params.Current,
			}, nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: params.Current}, nil
		},
		// The answer is a path, which is the daemon's key; the recap shows the
		// branch, which is what the reader typed and recognises.
		Summarize: func(answer flow.Answer) string { return list.branchOf(answer.Value) },
	}
}

func worktreeOptions(worktrees []domain.GitWorktree, params WorktreeParams) []flow.Option {
	options := make([]flow.Option, 0, len(worktrees))
	for _, wt := range worktrees {
		var badges []flow.Badge
		if count := params.Running[wt.Path]; count > 0 {
			badges = append(badges, flow.Badge{
				Text: fmt.Sprintf(domain.RunWorktreeJobsFmt, count),
				Tone: domain.ToneSuccess,
			})
		}
		if wt.Path == params.Current {
			badges = append(badges, flow.Badge{Text: domain.RunWorktreeCurrent})
		}
		options = append(options, flow.Option{Label: wt.Branch, Value: wt.Path, Badges: badges})
	}
	return options
}

type JobParams struct {
	Jobs []domain.JobConfig
	// Flag is what an unattended run is told to pass, since there is no safe
	// default for "which job": naming one is the whole request.
	Flag string
}

// JobStep asks which job to act on. It has no Resolve: a run that cannot ask is
// refused naming the flag rather than falling back to a picker.
func JobStep(params JobParams) flow.Step {
	return flow.Step{
		Kind:  flow.StepSelect,
		Key:   KeyJob,
		Label: domain.RunJobStepName,
		Flag:  params.Flag,
		Build: func(flow.Answers) (flow.StepContent, error) {
			if len(params.Jobs) == 0 {
				return flow.StepContent{}, domain.ErrNoJobsDeclared
			}
			return flow.StepContent{
				Title:   domain.RunJobPickerTitle,
				Options: jobOptions(params.Jobs),
			}, nil
		},
	}
}

func jobOptions(jobs []domain.JobConfig) []flow.Option {
	options := make([]flow.Option, 0, len(jobs))
	for _, job := range jobs {
		var badges []flow.Badge
		if job.Kind != "" {
			badges = append(badges, flow.Badge{Text: string(job.Kind)})
		}
		options = append(options, flow.Option{Label: job.Name, Value: job.Name, Badges: badges})
	}
	return options
}

type ProfileParams struct {
	Profiles []domain.ProfileConfig
	// Default is the profile a run takes when it is not asked, empty for a config
	// declaring none.
	Default string
}

// ProfileStep asks which profile to start. Unlike the job, it has a safe
// default — the config's default profile — so it resolves rather than refuses,
// and a config with one profile or none is never asked at all.
func ProfileStep(params ProfileParams) flow.Step {
	return flow.Step{
		Kind:  flow.StepSelect,
		Key:   KeyProfile,
		Label: domain.RunProfileStepName,
		Skip: func(flow.Answers) (bool, string) {
			if len(params.Profiles) <= 1 {
				return true, domain.RunProfileNoChoice
			}
			return false, ""
		},
		Build: func(flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{
				Title:       domain.RunProfilePickerTitle,
				Description: domain.RunProfilePickerDesc,
				Options:     profileOptions(params.Profiles),
				Start:       params.Default,
			}, nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: params.Default}, nil
		},
	}
}

func profileOptions(profiles []domain.ProfileConfig) []flow.Option {
	options := make([]flow.Option, 0, len(profiles))
	for _, profile := range profiles {
		label := profile.Name
		if len(profile.Jobs) > 0 {
			label += fmt.Sprintf(" (%s)", strings.Join(profile.Jobs, domain.RunURLListSep))
		}
		options = append(options, flow.Option{Label: label, Value: profile.Name})
	}
	return options
}
