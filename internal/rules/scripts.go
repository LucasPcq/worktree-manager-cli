package rules

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

var scriptServiceKeywords = []string{
	domain.ScriptKeyDev,
	domain.ScriptKeyStart,
	domain.ScriptKeyServe,
	domain.ScriptKeyWatch,
}

// StripScope removes an npm scope prefix from a package name.
// "@acme/web" → "web", "my-app" → "my-app".
func StripScope(name string) string {
	if !strings.HasPrefix(name, "@") {
		return name
	}
	parts := strings.SplitN(name, "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return name
}

// ClassifyScriptKind returns JobKindService for long-running dev scripts and
// JobKindTask for everything else.
//
// A script name maps to JobKindService when, for any keyword kw in
// {dev, start, serve, watch}, the name satisfies at least one condition:
//   - exact match: name == kw
//   - prefix:      name starts with kw+":"   (e.g. "dev:api")
//   - suffix:      name ends with   ":"+kw   (e.g. "api:dev")
func ClassifyScriptKind(scriptName string) domain.JobKind {
	for _, kw := range scriptServiceKeywords {
		if scriptName == kw {
			return domain.JobKindService
		}
		if strings.HasPrefix(scriptName, kw+":") {
			return domain.JobKindService
		}
		if strings.HasSuffix(scriptName, ":"+kw) {
			return domain.JobKindService
		}
	}
	return domain.JobKindTask
}
