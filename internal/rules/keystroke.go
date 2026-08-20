package rules

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// keySequences is what a terminal writes for the keys that are not text. The
// letter-and-control keys are computed instead: ctrl+a through ctrl+z are the
// first 26 bytes, in order.
var keySequences = map[string]string{
	"enter":     "\r",
	"tab":       "\t",
	"shift+tab": "\x1b[Z",
	"esc":       "\x1b",
	"backspace": "\x7f",
	"delete":    "\x1b[3~",
	"insert":    "\x1b[2~",
	" ":         " ",
	"up":        "\x1b[A",
	"down":      "\x1b[B",
	"right":     "\x1b[C",
	"left":      "\x1b[D",
	"home":      "\x1b[H",
	"end":       "\x1b[F",
	"pgup":      "\x1b[5~",
	"pgdown":    "\x1b[6~",
	"ctrl+@":    "\x00",
	"ctrl+\\":   "\x1c",
	"ctrl+]":    "\x1d",
	"ctrl+^":    "\x1e",
	"ctrl+_":    "\x1f",
	"f1":        "\x1bOP",
	"f2":        "\x1bOQ",
	"f3":        "\x1bOR",
	"f4":        "\x1bOS",
	"f5":        "\x1b[15~",
	"f6":        "\x1b[17~",
	"f7":        "\x1b[18~",
	"f8":        "\x1b[19~",
	"f9":        "\x1b[20~",
	"f10":       "\x1b[21~",
	"f11":       "\x1b[23~",
	"f12":       "\x1b[24~",
}

// EncodeKeyStroke turns a keypress back into the bytes a terminal would have
// sent, for a surface handing its keyboard to a child process. A key it has no
// encoding for produces nothing rather than a guess: a stray byte in a job's
// stdin is worse than a key that did not reach it.
func EncodeKeyStroke(stroke domain.KeyStroke) []byte {
	encoded := encodeKeyName(stroke)
	if len(encoded) == 0 {
		return nil
	}
	if stroke.Alt {
		return append([]byte(domain.EscapePrefix), encoded...)
	}
	return encoded
}

func encodeKeyName(stroke domain.KeyStroke) []byte {
	if stroke.Name == "" {
		return []byte(string(stroke.Runes))
	}
	if sequence, named := keySequences[stroke.Name]; named {
		return []byte(sequence)
	}
	if control, ok := controlByte(stroke.Name); ok {
		return []byte{control}
	}
	return nil
}

// controlByte reads the ctrl+<letter> keys, and only those: ctrl+home and the
// other named combinations share the prefix without sharing the encoding.
func controlByte(name string) (byte, bool) {
	letter, found := strings.CutPrefix(name, domain.KeyCtrlPrefix)
	if !found || len(letter) != 1 || letter[0] < 'a' || letter[0] > 'z' {
		return 0, false
	}
	return letter[0] - 'a' + 1, true
}
