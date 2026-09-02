package run

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/tui/runpicker"
)

// askWizard is a variable so a test can answer for the wizard without a
// terminal, the way showRunView stands in for the full-screen view.
var askWizard = runpicker.RunTargetWizard

// secondAxis is what a command acts on inside the worktree: a job, a profile, or
// one of the jobs that publish a URL. Exactly one list is set.
type secondAxis struct {
	// Given is the flag's value. Non-empty means the question is already
	// answered, so it is not asked and does not appear in the breadcrumb.
	Given    string
	Jobs     []domain.JobConfig
	Profiles []domain.ProfileConfig
	URLs     []domain.JobURLEntry
	// ResolveURLs recomputes URLs for the worktree chosen in the same form.
	ResolveURLs func(worktreePath string) []domain.JobURLEntry
	Start       string
	// Required refuses the run when the answer is missing and nobody can be
	// asked. An axis with a safe default — the default profile — leaves it false
	// and comes back empty.
	Required bool
}

func (a secondAxis) asked() bool {
	return a.Given == "" && (len(a.Jobs) > 0 || len(a.Profiles) > 0 || len(a.URLs) > 0)
}

type inputsParams struct {
	// Args is the command's positional slice; its first entry, when present,
	// names the worktree.
	Args        []string
	Cwd         string
	ProjectDir  string
	Interactive bool
	// Pick allows the wizard. `run url` sets it false: it answers from its flags
	// or errors, and a form opening inside $(…) would hang the shell.
	Pick   bool
	Second secondAxis
}

type inputs struct {
	Dir    string
	Branch string
	Second string
}

// resolveInputs answers every question a run command has, in one place and — when
// more than one is outstanding — in one form. Asking them as separate pickers is
// what made cancelling the second throw away the answer to the first instead of
// stepping back to it.
//
// The worktree is a decision with a safe default, the current directory, so the
// non-interactive path never errors for want of it. The second axis has one only
// when the command says so.
func resolveInputs(params inputsParams) (inputs, error) {
	named, err := namedTarget(params)
	if err != nil {
		return inputs{}, err
	}
	if !params.Interactive || !params.Pick {
		return unattendedInputs(params, named)
	}

	worktreeStep, err := worktreeQuestion(params, named == nil)
	if err != nil {
		return inputs{}, err
	}
	secondStep := secondQuestion(params.Second)
	if worktreeStep == nil && secondStep == nil {
		return unattendedInputs(params, named)
	}

	answers, err := askWizard(runpicker.TargetWizardParams{Worktree: worktreeStep, Second: secondStep})
	if err != nil {
		return inputs{}, err
	}
	return merge(params, named, answers), nil
}

// namedTarget resolves the positional, and reports nil when there was none.
func namedTarget(params inputsParams) (*inputs, error) {
	query := firstArg(params.Args)
	if query == "" {
		return nil, nil
	}

	result, err := worktree.Resolve(domain.ResolveParams{ProjectDir: params.ProjectDir, Query: query})
	if err != nil {
		return nil, fmt.Errorf("worktree %q: %w", query, err)
	}
	if result.Ambiguous {
		return nil, fmt.Errorf("worktree %q is ambiguous: matches %s", query, branchNames(result.Matches))
	}
	return &inputs{Dir: result.Path, Branch: result.Branch}, nil
}

// unattendedInputs is the answer when nobody can be asked: the named worktree or
// the current one, and the second axis from its flag alone.
func unattendedInputs(params inputsParams, named *inputs) (inputs, error) {
	if params.Second.Given == "" && params.Second.Required {
		return inputs{}, domain.ErrJobRequired
	}
	return withSecond(params, named, ""), nil
}

// worktreeQuestion is the worktree step, or nil when the command named one or
// the repository holds a single worktree — a list of one asks nothing.
func worktreeQuestion(params inputsParams, ask bool) (*runpicker.WorktreeStep, error) {
	if !ask {
		return nil, nil
	}

	worktrees, err := worktree.ListAll(worktree.ListAllParams{ProjectDir: params.ProjectDir})
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}
	if len(worktrees) <= 1 {
		return nil, nil
	}

	return &runpicker.WorktreeStep{
		Worktrees: worktrees,
		Current:   worktreeRoot(params.Cwd),
		Running:   rules.RunningJobsByWorktree(shared.LoadJobsGraceful()),
	}, nil
}

func secondQuestion(axis secondAxis) *runpicker.SecondStep {
	if !axis.asked() {
		return nil
	}
	return &runpicker.SecondStep{
		Jobs:        axis.Jobs,
		Profiles:    axis.Profiles,
		URLs:        axis.URLs,
		ResolveURLs: axis.ResolveURLs,
		Start:       axis.Start,
	}
}

// merge folds the wizard's answers into what was already known: a question that
// never became a step keeps the value that spared it.
func merge(params inputsParams, named *inputs, answers runpicker.TargetWizardResult) inputs {
	if answers.WorktreePath != "" {
		named = &inputs{Dir: answers.WorktreePath}
	}
	return withSecond(params, named, answers.Second)
}

func withSecond(params inputsParams, named *inputs, answered string) inputs {
	resolved := inputs{Dir: worktreeRoot(params.Cwd)}
	if named != nil {
		resolved = *named
	}
	resolved.Second = params.Second.Given
	if answered != "" {
		resolved.Second = answered
	}
	return resolved
}

// worktreeRoot is the worktree containing dir, spelled the way git spells it.
// Never dir itself: the directory a command was launched from doubles as the
// daemon's key for a job (name + WorkDir) and as the job's own working
// directory, which run.toml's `cwd` is resolved against. A subdirectory would
// therefore both mis-resolve that `cwd` and make the same worktree two distinct
// keys, so `run down` could not find what `run up` had started.
func worktreeRoot(dir string) string {
	root, err := infra.Toplevel(dir)
	if err != nil {
		return dir
	}
	return root
}

// declaredJob turns an answered job name into its declaration, so a name matching
// nothing in run.toml fails precisely instead of at the daemon.
func declaredJob(cfg domain.RunConfig, name string) (domain.JobConfig, error) {
	if name == "" {
		return domain.JobConfig{}, domain.ErrJobRequired
	}
	job, ok := rules.FindJob(cfg, name)
	if !ok {
		return domain.JobConfig{}, fmt.Errorf("%w: %s", domain.ErrJobNotFound, name)
	}
	return job, nil
}

func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

func branchNames(worktrees []domain.GitWorktree) string {
	names := make([]string, 0, len(worktrees))
	for _, wt := range worktrees {
		names = append(names, wt.Branch)
	}
	return strings.Join(names, domain.RunURLListSep)
}
