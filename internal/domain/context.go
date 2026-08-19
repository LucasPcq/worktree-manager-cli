package domain

// ProjectContext carries the loaded config and the two paths every command
// resolves from the cwd: ProjectDir is the main worktree (git ops, BasePath
// resolution), StateDir holds the wtm config / run / metadata files.
type ProjectContext struct {
	ProjectDir string
	StateDir   string
	Config     Config
}
