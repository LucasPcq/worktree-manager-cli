package domain

// EnvLineKind classifies a logical line of a .env document.
type EnvLineKind string

const (
	EnvLinePair    EnvLineKind = "pair"
	EnvLineComment EnvLineKind = "comment"
	EnvLineBlank   EnvLineKind = "blank"
)

// EnvLine is one logical line of a .env document. Raw holds the exact original
// text (newline excluded) and is the source of truth for rendering while present;
// mutating Key/Value requires clearing Raw so RenderEnv re-emits the line
// canonically. Key, Value and Export are meaningful only for EnvLinePair; Value is
// opaque (inner text, unquoted, no expansion or escape interpretation).
type EnvLine struct {
	Kind   EnvLineKind
	Key    string
	Value  string
	Export bool
	Raw    string
}
