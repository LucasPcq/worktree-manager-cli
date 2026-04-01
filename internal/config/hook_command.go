package config

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

// unmarshalHookCommand decodes a TOML value that is either a plain string
// or a table with {cmd, cwd} into a domain.HookCommand.
func unmarshalHookCommand(raw interface{}) (domain.HookCommand, error) {
	s, ok := raw.(string)
	if ok {
		return domain.HookCommand{Cmd: s}, nil
	}

	table, ok := raw.(map[string]interface{})
	if !ok {
		return domain.HookCommand{}, fmt.Errorf("hook must be a string or {cmd, cwd}, got %T", raw)
	}

	cmd, ok := table["cmd"].(string)
	if !ok {
		return domain.HookCommand{}, fmt.Errorf("hook table requires a string \"cmd\" field")
	}

	h := domain.HookCommand{Cmd: cmd}

	cwd, ok := table["cwd"].(string)
	if ok {
		h.Cwd = cwd
	}

	return h, nil
}

// unmarshalHookCommands decodes a TOML array of mixed string/table hook entries.
func unmarshalHookCommands(raw []interface{}) ([]domain.HookCommand, error) {
	hooks := make([]domain.HookCommand, 0, len(raw))
	for i, entry := range raw {
		h, err := unmarshalHookCommand(entry)
		if err != nil {
			return nil, fmt.Errorf("hook[%d]: %w", i, err)
		}
		hooks = append(hooks, h)
	}
	return hooks, nil
}
