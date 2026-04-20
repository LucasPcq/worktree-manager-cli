package domain

import "strings"

var scriptServiceKeywords = []string{
	ScriptKeyDev,
	ScriptKeyStart,
	ScriptKeyServe,
	ScriptKeyWatch,
}

// ClassifyScriptKind returns JobKindService for long-running dev scripts and
// JobKindTask for everything else.
//
// A script name maps to JobKindService when, for any keyword kw in
// {dev, start, serve, watch}, the name satisfies at least one condition:
//   - exact match: name == kw
//   - prefix:      name starts with kw+":"   (e.g. "dev:api")
//   - suffix:      name ends with   ":"+kw   (e.g. "api:dev")
func ClassifyScriptKind(scriptName string) JobKind {
	for _, kw := range scriptServiceKeywords {
		if scriptName == kw {
			return JobKindService
		}
		if strings.HasPrefix(scriptName, kw+":") {
			return JobKindService
		}
		if strings.HasSuffix(scriptName, ":"+kw) {
			return JobKindService
		}
	}
	return JobKindTask
}
