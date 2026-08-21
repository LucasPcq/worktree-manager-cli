package domain

// ComposePortStatus says what a compose port mapping allows wtm to do with it.
type ComposePortStatus string

const (
	// ComposePortTemplated is a mapping whose host side already reads an
	// environment variable ("${DB_PORT:-5432}:5432"). Nothing to rewrite: the
	// variable name and its default are taken as declared.
	ComposePortTemplated ComposePortStatus = "templated"

	// ComposePortFrozen is a literal host port ("5432:5432"). Declaring it in
	// run.toml changes nothing until the mapping reads a variable, so it is the
	// only status wtm offers to rewrite.
	ComposePortFrozen ComposePortStatus = "frozen"

	// ComposePortUnsupported is a mapping wtm refuses to touch — a range, an
	// alias, or a form with no host port to shift. Reason says which.
	ComposePortUnsupported ComposePortStatus = "unsupported"
)

// ComposePortBinding is one entry of a service's `ports:` list, located in the
// source file precisely enough to be rewritten without re-serializing it.
type ComposePortBinding struct {
	File    string
	Service string
	Status  ComposePortStatus
	// Reason is filled for ComposePortUnsupported only.
	Reason string
	// Var is the environment variable the host port reads: the one already
	// written for a templated mapping, the one wtm would introduce for a frozen
	// one. Empty when unsupported.
	Var string
	// Base is the host port the main checkout binds — the declared literal, or
	// the default of a templated mapping.
	Base int
	// Container is the port inside the container. It never shifts.
	Container int
	// Line and Column locate the scalar in the source file, 1-based, Column
	// pointing at its first character, opening quote included.
	Line   int
	Column int
	// Token is the scalar exactly as the file spells it, quotes included, and
	// Replacement what it becomes when patched. Replacement is empty unless
	// Status is ComposePortFrozen.
	Token       string
	Replacement string
}

// ComposeScan is everything one docker-compose file says about its ports.
type ComposeScan struct {
	File string
	// Err is set when the file could not be read or parsed at all; Bindings is
	// then empty and the file is reported rather than used.
	Err      string
	Bindings []ComposePortBinding
}
