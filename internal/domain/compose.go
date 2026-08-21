package domain

// ComposePortStatus says what a compose port mapping allows wtm to do with it.
// Only a frozen one is ever rewritten: a templated mapping already reads a
// variable, and an unsupported one wtm refuses to touch.
type ComposePortStatus string

const (
	ComposePortTemplated   ComposePortStatus = "templated"
	ComposePortFrozen      ComposePortStatus = "frozen"
	ComposePortUnsupported ComposePortStatus = "unsupported"
)

// ComposePortBinding is one entry of a service's `ports:` list, located in the
// source file precisely enough to be rewritten without re-serializing it.
type ComposePortBinding struct {
	File    string
	Service string
	Status  ComposePortStatus
	Reason  string
	Var     string
	// Base is the host port the main checkout binds; Container never shifts.
	Base      int
	Container int
	// Line and Column are 1-based, Column pointing at the scalar's first
	// character, opening quote included.
	Line   int
	Column int
	// Token is the scalar exactly as the file spells it, quotes included, and
	// Replacement what it becomes — empty unless Status is ComposePortFrozen.
	Token       string
	Replacement string
}

// ComposeScan is everything one docker-compose file says about its ports. Err
// is set when the file could not be read or parsed at all, and Bindings is then
// empty: the file is reported rather than used.
type ComposeScan struct {
	File     string
	Err      string
	Bindings []ComposePortBinding
}
