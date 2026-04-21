package rules

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// TemplateVars holds the variables available for interpolation in hook commands.
type TemplateVars struct {
	Worktree   string
	Branch     string
	Root       string
	FromBranch string
}

// ResolveTemplateVars substitutes template variables in a hook command's Cmd and Cwd fields.
func ResolveTemplateVars(hook domain.HookCommand, vars TemplateVars) domain.HookCommand {
	hook.Cmd = Interpolate(hook.Cmd, vars)
	hook.Cwd = Interpolate(hook.Cwd, vars)
	return hook
}

// Interpolate replaces {{worktree}}, {{branch}}, {{root}}, {{from_branch}} in s.
func Interpolate(s string, vars TemplateVars) string {
	if s == "" {
		return s
	}
	r := strings.NewReplacer(
		"{{worktree}}", vars.Worktree,
		"{{branch}}", vars.Branch,
		"{{root}}", vars.Root,
		"{{from_branch}}", vars.FromBranch,
	)
	return r.Replace(s)
}
