package rules

import (
	"regexp"
	"sort"
	"strconv"

	"github.com/LucasPcq/wtm/internal/domain"
)

var envVarNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// IsEnvVarName reports whether key can be exported to a child process.
func IsEnvVarName(key string) bool { return envVarNamePattern.MatchString(key) }

// EffectivePortOffsetBlock is the spacing a config actually runs with. A block
// left unset, or set to zero, is the default rather than a block of nothing —
// which would collapse every worktree onto the main checkout's ports.
func EffectivePortOffsetBlock(cfg domain.RunConfig) int {
	if cfg.PortOffsetBlock <= 0 {
		return domain.PortOffsetBlock
	}
	return cfg.PortOffsetBlock
}

type JobPortsParams struct {
	Ports      map[string]int
	PortOffset int
}

// JobPorts resolves a job's declared base ports for the worktree it is about to
// run in. The main checkout has offset 0, so its ports stay exactly as declared.
func JobPorts(params JobPortsParams) map[string]int {
	if len(params.Ports) == 0 {
		return nil
	}
	resolved := make(map[string]int, len(params.Ports))
	for name, base := range params.Ports {
		resolved[name] = base + params.PortOffset
	}
	return resolved
}

// JobPortEnv is JobPorts as a child process reads it.
func JobPortEnv(params JobPortsParams) map[string]string {
	resolved := JobPorts(params)
	if resolved == nil {
		return nil
	}
	env := make(map[string]string, len(resolved))
	for name, port := range resolved {
		env[name] = strconv.Itoa(port)
	}
	return env
}

// PortDeclaration is one `NAME = base` entry, carrying the job it came from so
// a collision can name both sides.
type PortDeclaration struct {
	Job  string
	Name string
	Base int
}

// PortCollision is a pair of declarations that end up on the same port.
// Worktrees is how far apart the two worktrees reaching it are — zero meaning
// the pair already collides inside a single one.
type PortCollision struct {
	A         PortDeclaration
	B         PortDeclaration
	Worktrees int
}

// PortDeclarations flattens a config's ports in a stable order: jobs as
// declared, names sorted within each job.
func PortDeclarations(cfg domain.RunConfig) []PortDeclaration {
	var decls []PortDeclaration
	for _, job := range cfg.Jobs {
		for _, name := range sortedPortNames(job.Ports) {
			decls = append(decls, PortDeclaration{Job: job.Name, Name: name, Base: job.Ports[name]})
		}
	}
	return decls
}

// PortCollisions finds the base ports that cannot coexist. Two of them meet
// whenever their difference is a multiple of the block: worktree n binds
// base+n×block, so a gap of k×block is exactly what k worktrees of offset
// close. The horizon bounds it — a pair that only meets on the 508th worktree
// is arithmetic, not a conflict.
func PortCollisions(cfg domain.RunConfig) []PortCollision {
	block := EffectivePortOffsetBlock(cfg)
	decls := PortDeclarations(cfg)

	var collisions []PortCollision
	for i := range decls {
		for j := i + 1; j < len(decls); j++ {
			gap := decls[j].Base - decls[i].Base
			if gap < 0 {
				gap = -gap
			}
			if gap%block != 0 {
				continue
			}
			if apart := gap / block; apart <= domain.PortCollisionHorizon {
				collisions = append(collisions, PortCollision{A: decls[i], B: decls[j], Worktrees: apart})
			}
		}
	}
	return collisions
}

func sortedPortNames(ports map[string]int) []string {
	names := make([]string, 0, len(ports))
	for name := range ports {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
