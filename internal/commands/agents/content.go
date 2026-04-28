package agents

import _ "embed"

// usingWtmSkill is the canonical using-wtm skill file shipped with wtm. It is
// copied verbatim to every .claude / .cursor skills/ destination selected by
// `wtm agents install`.
//
//go:embed assets/using-wtm.skill.md
var usingWtmSkill string

// renderSkillMarkdown returns the bytes we drop at the skill destination.
func renderSkillMarkdown() string {
	return usingWtmSkill
}
