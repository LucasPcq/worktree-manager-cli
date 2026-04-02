package commands

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
)

func loadConfig(cmd *cobra.Command, dir string) (domain.Config, bool) {
	cfg, err := config.Load(config.LoadParams{ProjectDir: dir})
	if errors.Is(err, domain.ErrConfigNotFound) {
		fmt.Fprintln(cmd.ErrOrStderr(), "No .wtm.toml found. Run `wtm init` first.")
		return domain.Config{}, false
	}
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error loading config: %v\n", err)
		return domain.Config{}, false
	}
	return cfg, true
}
