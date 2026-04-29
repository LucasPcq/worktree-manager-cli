package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

// decodeStrict parses path into out and rejects any TOML key that did not map
// to a struct field. This catches typos like `[[profiles]]` (plural) when the
// schema expects `[[profile]]` (singular) — without this the toml library
// would silently ignore the unknown section.
//
// Optional ignorePrefixes mark sub-trees that the strict check should skip,
// useful when a struct intentionally decodes into `interface{}` (the toml
// library reports those inner keys as undecoded even though they were read
// into a map). Pass a prefix without a trailing dot, e.g. "hooks.on_create".
func decodeStrict(path string, out any, ignorePrefixes ...string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return decodeStrictBytes(path, data, out, ignorePrefixes...)
}

// decodeStrictBytes is the in-memory variant of decodeStrict, useful for
// tests and inline configs.
func decodeStrictBytes(source string, data []byte, out any, ignorePrefixes ...string) error {
	md, err := toml.Decode(string(data), out)
	if err != nil {
		return fmt.Errorf("parse %s: %w", source, err)
	}
	var keys []string
	for _, k := range md.Undecoded() {
		s := k.String()
		if hasIgnorablePrefix(s, ignorePrefixes) {
			continue
		}
		keys = append(keys, s)
	}
	if len(keys) > 0 {
		return fmt.Errorf("unknown keys in %s: %s", source, strings.Join(keys, ", "))
	}
	return nil
}

func hasIgnorablePrefix(key string, prefixes []string) bool {
	for _, p := range prefixes {
		if key == p || strings.HasPrefix(key, p+".") {
			return true
		}
	}
	return false
}
