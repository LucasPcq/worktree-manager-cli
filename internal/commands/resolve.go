package commands

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	gopicker "github.com/LucasPcq/wtm/internal/tui/go"
)

// NewResolveCmd creates the wtm resolve command (internal, used by shell wrapper).
func NewResolveCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "resolve [branch]",
		Short:  "Resolve a branch to its worktree path",
		Hidden: true,
		RunE:   runResolve,
	}
}

func runResolve(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	root, err := projectRoot(dir)
	if err != nil {
		return err
	}

	query := strings.Join(args, " ")

	result, err := worktree.Resolve(worktree.ResolveParams{
		ProjectDir: root,
		Query:      query,
	})
	if errors.Is(err, domain.ErrWorktreeNotFound) {
		output.Blank(cmd.ErrOrStderr())
		output.Warning(cmd.ErrOrStderr(), fmt.Sprintf("No worktree found matching %q", query))
		output.Blank(cmd.ErrOrStderr())
		return nil
	}
	if err != nil {
		return err
	}

	if result.Ambiguous {
		path, pickerErr := gopicker.RunWorktreePicker(result.Matches)
		if errors.Is(pickerErr, domain.ErrUserAborted) {
			return nil
		}
		if pickerErr != nil {
			return pickerErr
		}
		fmt.Fprintln(cmd.OutOrStdout(), path)
		return nil
	}

	fmt.Fprintln(cmd.OutOrStdout(), result.Path)
	return nil
}
