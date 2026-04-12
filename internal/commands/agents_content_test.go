package commands

import (
	"strings"
	"testing"
)

func TestRenderSkillMarkdown_HasFrontmatter(t *testing.T) {
	got := renderSkillMarkdown()
	if !strings.HasPrefix(got, "---\nname: using-wtm\n") {
		t.Errorf("missing or wrong frontmatter:\n%s", got[:120])
	}
	if !strings.Contains(got, "# Using wtm") {
		t.Error("expected H1 heading in skill body")
	}
	if !strings.Contains(got, "--output json") {
		t.Error("skill body should mention --output json")
	}
}
