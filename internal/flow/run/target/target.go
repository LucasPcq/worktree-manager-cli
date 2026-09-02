// Package target holds the questions every run command shares: which worktree
// it acts on, and what inside it — a job or a profile. They live here rather
// than in each command's package for the reason internal/flow/decide exists:
// one wording, one bypass taxonomy, one place a surface has to learn.
package target

import (
	"fmt"
	"strings"
	"sync"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

const (
	KeyWorktree = "run.worktree"
	KeyJob      = "run.job"
	KeyProfile  = "run.profile"
)

type ResolveParams struct {
	ProjectDir string
	// Query is the positional as it was typed: a branch, a path, a prefix.
	Query string
}

// Resolved is a named worktree: where it is, and the branch it carries.
type Resolved struct {
	Dir    string
	Branch string
}

// Resolve turns the positional into a worktree, and refuses a name matching
// none or several. It is the flow's job rather than the runner's: naming a
// worktree is a business input, and the two surfaces must refuse the same names.
func Resolve(params ResolveParams) (Resolved, error) {
	result, err := worktree.Resolve(domain.ResolveParams{ProjectDir: params.ProjectDir, Query: params.Query})
	if err != nil {
		return Resolved{}, fmt.Errorf("worktree %q: %w", params.Query, err)
	}
	if result.Ambiguous {
		return Resolved{}, fmt.Errorf("worktree %q is ambiguous: matches %s", params.Query, branchNames(result.Matches))
	}
	return Resolved{Dir: result.Path, Branch: result.Branch}, nil
}

// Root is the worktree containing dir as git spells it, and dir itself when git
// cannot say — a path outside any repository is refused later, by the command
// that needed a worktree, rather than here.
func Root(dir string) string {
	root, err := worktree.Root(dir)
	if err != nil {
		return dir
	}
	return root
}

// RunningJobs counts what the daemon holds per worktree, and answers nothing
// when it cannot be reached: the counts decorate a picker, they never gate it.
func RunningJobs(service runlogs.Service) map[string]int {
	if service == nil {
		return nil
	}
	jobs, err := service.List("")
	if err != nil {
		return nil
	}
	return rules.RunningJobsByWorktree(jobs)
}

// DeclaredJob turns an answered job name into its declaration, so a name
// matching nothing in run.toml fails precisely instead of at the daemon.
func DeclaredJob(cfg domain.RunConfig, name string) (domain.JobConfig, error) {
	if name == "" {
		return domain.JobConfig{}, domain.ErrJobRequired
	}
	job, ok := rules.FindJob(cfg, name)
	if !ok {
		return domain.JobConfig{}, fmt.Errorf("%w: %s", domain.ErrJobNotFound, name)
	}
	return job, nil
}

func branchNames(worktrees []domain.GitWorktree) string {
	names := make([]string, 0, len(worktrees))
	for _, wt := range worktrees {
		names = append(names, wt.Branch)
	}
	return strings.Join(names, domain.RunURLListSep)
}

// worktreeList reads the repository's worktrees once, however many of the step's
// callbacks ask for them. A failure is remembered too: it means "do not ask",
// and the step's Resolve then answers with the current worktree.
type worktreeList struct {
	projectDir string
	once       sync.Once
	worktrees  []domain.GitWorktree
	err        error
}

func (l *worktreeList) get() ([]domain.GitWorktree, error) {
	l.once.Do(func() {
		l.worktrees, l.err = worktree.ListAll(worktree.ListAllParams{ProjectDir: l.projectDir})
	})
	return l.worktrees, l.err
}

func (l *worktreeList) branchOf(path string) string {
	worktrees, err := l.get()
	if err != nil {
		return path
	}
	for _, wt := range worktrees {
		if wt.Path == path {
			return wt.Branch
		}
	}
	return path
}

// Named resolves the positional, and reports nil when there was none — which is
// the difference between "this worktree" and "no worktree named yet".
func Named(params ResolveParams) (*Resolved, error) {
	if params.Query == "" {
		return nil, nil
	}
	resolved, err := Resolve(params)
	if err != nil {
		return nil, err
	}
	return &resolved, nil
}

type WorkDirParams struct {
	Answers flow.Answers
	Named   *Resolved
	Cwd     string
}

// WorkDir is the worktree a run acts on, as git spells it: what the step
// answered, else the positional, else where the command was launched.
func WorkDir(params WorkDirParams) string {
	if answered := params.Answers.Value(KeyWorktree); answered != "" {
		return answered
	}
	if params.Named != nil {
		return params.Named.Dir
	}
	return Root(params.Cwd)
}

type PresetParams struct {
	Named   *Resolved
	Job     string
	Profile string
}

// Presets carry what the flags and the positional already answered. A preset
// step is not asked but is still read back, which is what keeps a flag from
// erasing a line from the recap.
func Presets(params PresetParams) flow.Answers {
	values := map[string]string{KeyJob: params.Job, KeyProfile: params.Profile}
	if params.Named != nil {
		values[KeyWorktree] = params.Named.Dir
	}
	return flow.NewAnswers(values)
}
