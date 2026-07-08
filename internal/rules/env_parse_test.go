package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestParseRenderRoundTrip(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"simple pair with newline", "KEY=value\n"},
		{"no trailing newline", "KEY=value"},
		{"export prefix", "export KEY=value\n"},
		{"leading indentation", "  KEY=value\n"},
		{"double quoted", "KEY=\"hello world\"\n"},
		{"single quoted", "KEY='hello world'\n"},
		{"value containing equals", "URL=postgres://user:pw@host/db?a=b\n"},
		{"hash inside quotes", "KEY=\"a # b\"\n"},
		{"full and inline comments", "# header\nKEY=value # trailing\n"},
		{"blank line between pairs", "A=1\n\nB=2\n"},
		{"multiple trailing blanks", "A=1\n\n\n"},
		{"multiline quoted value", "CERT=\"-----BEGIN-----\nMIIBcontent\n-----END-----\"\n"},
		{"multiline json value", "CONFIG='{\n  \"a\": 1\n}'\n"},
		{"degenerate no equals", "just some prose\nKEY=1\n"},
		{"crlf line endings", "A=1\r\nB=2\r\n"},
		{"empty content", ""},
		{"only blank lines", "\n\n"},
		{"empty value", "KEY=\n"},
		{"internal whitespace unquoted", "KEY=hello world\n"},
		{"escaped quote in double quotes", "KEY=\"he said \\\"hi\\\"\"\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := RenderEnv(ParseEnv(c.content))
			if got != c.content {
				t.Errorf("round-trip mismatch\n input: %q\noutput: %q", c.content, got)
			}
		})
	}
}

func TestParseEnvClassification(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    domain.EnvLine
	}{
		{
			"plain pair",
			"KEY=value",
			domain.EnvLine{Kind: domain.EnvLinePair, Key: "KEY", Value: "value", Raw: "KEY=value"},
		},
		{
			"export pair",
			"export FOO=bar",
			domain.EnvLine{Kind: domain.EnvLinePair, Key: "FOO", Value: "bar", Export: true, Raw: "export FOO=bar"},
		},
		{
			"hash inside quotes is not a comment",
			"KEY=\"a # b\"",
			domain.EnvLine{Kind: domain.EnvLinePair, Key: "KEY", Value: "a # b", Raw: "KEY=\"a # b\""},
		},
		{
			"inline comment stripped from value",
			"KEY=val # comment",
			domain.EnvLine{Kind: domain.EnvLinePair, Key: "KEY", Value: "val", Raw: "KEY=val # comment"},
		},
		{
			"only first equals splits key",
			"URL=postgres://a=b",
			domain.EnvLine{Kind: domain.EnvLinePair, Key: "URL", Value: "postgres://a=b", Raw: "URL=postgres://a=b"},
		},
		{
			"single quoted value",
			"KEY='raw value'",
			domain.EnvLine{Kind: domain.EnvLinePair, Key: "KEY", Value: "raw value", Raw: "KEY='raw value'"},
		},
		{
			"full line comment",
			"# a comment",
			domain.EnvLine{Kind: domain.EnvLineComment, Raw: "# a comment"},
		},
		{
			"blank line",
			"",
			domain.EnvLine{Kind: domain.EnvLineBlank, Raw: ""},
		},
		{
			"degenerate line is a comment",
			"just some prose",
			domain.EnvLine{Kind: domain.EnvLineComment, Raw: "just some prose"},
		},
		{
			"leading digit key is degenerate",
			"1KEY=value",
			domain.EnvLine{Kind: domain.EnvLineComment, Raw: "1KEY=value"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lines := ParseEnv(c.content)
			if len(lines) != 1 {
				t.Fatalf("ParseEnv(%q) = %d lines, want 1", c.content, len(lines))
			}
			if lines[0] != c.want {
				t.Errorf("ParseEnv(%q)[0] = %+v, want %+v", c.content, lines[0], c.want)
			}
		})
	}
}

func TestRenderEnvReformatOnChange(t *testing.T) {
	cases := []struct {
		name string
		line domain.EnvLine
		want string
	}{
		{
			"bare value needs no quotes",
			domain.EnvLine{Kind: domain.EnvLinePair, Key: "KEY", Value: "simple"},
			"KEY=simple",
		},
		{
			"value with space is quoted",
			domain.EnvLine{Kind: domain.EnvLinePair, Key: "KEY", Value: "hello world"},
			"KEY=\"hello world\"",
		},
		{
			"export prefix preserved",
			domain.EnvLine{Kind: domain.EnvLinePair, Key: "FOO", Value: "bar", Export: true},
			"export FOO=bar",
		},
		{
			"value with equals stays bare",
			domain.EnvLine{Kind: domain.EnvLinePair, Key: "KEY", Value: "a=b"},
			"KEY=a=b",
		},
		{
			"inline-comment-like value is quoted",
			domain.EnvLine{Kind: domain.EnvLinePair, Key: "KEY", Value: "val # x"},
			"KEY=\"val # x\"",
		},
		{
			"embedded double quote is escaped",
			domain.EnvLine{Kind: domain.EnvLinePair, Key: "KEY", Value: "say \"hi\""},
			"KEY=\"say \\\"hi\\\"\"",
		},
		{
			"empty value stays bare",
			domain.EnvLine{Kind: domain.EnvLinePair, Key: "KEY", Value: ""},
			"KEY=",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := RenderEnv([]domain.EnvLine{c.line})
			if got != c.want {
				t.Errorf("RenderEnv(%+v) = %q, want %q", c.line, got, c.want)
			}
		})
	}
}

func TestRenderEnvReformatKeepsNeighborsVerbatim(t *testing.T) {
	lines := []domain.EnvLine{
		{Kind: domain.EnvLineComment, Raw: "# keep me"},
		{Kind: domain.EnvLinePair, Key: "A", Value: "1", Raw: "A=1   # aligned"},
		{Kind: domain.EnvLinePair, Key: "B", Value: "new value"},
		{Kind: domain.EnvLineBlank, Raw: ""},
	}
	got := RenderEnv(lines)
	want := "# keep me\nA=1   # aligned\nB=\"new value\"\n"
	if got != want {
		t.Errorf("RenderEnv = %q, want %q", got, want)
	}
}
