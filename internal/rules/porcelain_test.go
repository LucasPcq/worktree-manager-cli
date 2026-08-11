package rules_test

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestParsePorcelainZ(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want []domain.PorcelainEntry
	}{
		{
			name: "modified file keeps its raw XY field",
			out:  " M a.txt\x00",
			want: []domain.PorcelainEntry{{Status: " M", Path: "a.txt"}},
		},
		{
			// -z disables path quoting: no surrounding quotes, no \303\251 escapes.
			name: "paths with spaces and non-ASCII come out verbatim",
			out:  "?? a b.txt\x00?? café.txt\x00",
			want: []domain.PorcelainEntry{
				{Status: "??", Path: "a b.txt"},
				{Status: "??", Path: "café.txt"},
			},
		},
		{
			name: "a new directory is listed file by file",
			out:  "?? newmod/x.go\x00?? newmod/sub/y.go\x00",
			want: []domain.PorcelainEntry{
				{Status: "??", Path: "newmod/x.go"},
				{Status: "??", Path: "newmod/sub/y.go"},
			},
		},
		{
			// The origin field must be consumed with its record, or every later
			// record is read off-by-one and turns to noise.
			name: "a rename carries its origin path and does not desync the stream",
			out:  "R  new.txt\x00old.txt\x00 M z.txt\x00",
			want: []domain.PorcelainEntry{
				{Status: "R ", Path: "new.txt", OrigPath: "old.txt"},
				{Status: " M", Path: "z.txt"},
			},
		},
		{
			name: "a copy behaves like a rename",
			out:  "C  copy.txt\x00src.txt\x00",
			want: []domain.PorcelainEntry{{Status: "C ", Path: "copy.txt", OrigPath: "src.txt"}},
		},
		{
			name: "a rename modified in the worktree is still a rename",
			out:  "RM new.txt\x00old.txt\x00",
			want: []domain.PorcelainEntry{{Status: "RM", Path: "new.txt", OrigPath: "old.txt"}},
		},
		{
			name: "ignored entries are dropped",
			out:  "!! build/out.bin\x00 M a.txt\x00",
			want: []domain.PorcelainEntry{{Status: " M", Path: "a.txt"}},
		},
		{name: "empty output", out: "", want: nil},
		{name: "separator only", out: "\x00", want: nil},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := rules.ParsePorcelainZ([]byte(c.out))
			if err != nil {
				t.Fatalf("ParsePorcelainZ: %v", err)
			}
			if len(got) != len(c.want) {
				t.Fatalf("got %+v, want %+v", got, c.want)
			}
			for i, w := range c.want {
				if got[i] != w {
					t.Errorf("entry %d = %+v, want %+v", i, got[i], w)
				}
			}
		})
	}
}

func TestParsePorcelainZMalformed(t *testing.T) {
	cases := map[string]string{
		"record shorter than a status field": "?\x00",
		"status field with no path":          "?? \x00",
		"rename with no origin field":        "R  new.txt\x00",
	}

	for name, out := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := rules.ParsePorcelainZ([]byte(out)); !errors.Is(err, domain.ErrPorcelainMalformed) {
				t.Fatalf("err = %v, want ErrPorcelainMalformed", err)
			}
		})
	}
}
