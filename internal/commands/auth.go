package commands

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	ghservice "github.com/LucasPcq/wtm/internal/service/github"
)

// NewAuthCmd creates the wtm auth command group.
func NewAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage GitHub authentication",
	}

	cmd.AddCommand(newAuthLoginCmd())
	cmd.AddCommand(newAuthStatusCmd())
	cmd.AddCommand(newAuthLogoutCmd())

	return cmd
}

func newAuthLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Authenticate with GitHub via Device Flow",
		RunE:  runAuthLogin,
	}
}

func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current authentication status",
		RunE:  runAuthStatus,
	}
}

func newAuthLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove stored GitHub credentials",
		RunE:  runAuthLogout,
	}
}

func runAuthLogin(cmd *cobra.Command, _ []string) error {
	existing, err := config.LoadAuth()
	if err == nil && existing.AccessToken != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "Already logged in as %s. Run `wtm auth logout` first.\n", existing.User)
		return nil
	}

	token, err := ghservice.DeviceFlowLogin(ghservice.DeviceFlowLoginParams{
		Output: cmd.ErrOrStderr(),
	})
	if err != nil {
		return err
	}

	if err := config.SaveAuth(token); err != nil {
		return fmt.Errorf("save token: %w", err)
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "✓ Logged in as %s\n", token.User)
	return nil
}

func runAuthStatus(cmd *cobra.Command, _ []string) error {
	token, err := config.LoadAuth()
	if errors.Is(err, domain.ErrAuthNotConfigured) {
		fmt.Fprintln(cmd.ErrOrStderr(), "Not authenticated. Run `wtm auth login` to connect.")
		return nil
	}
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "✓ Authenticated as %s\n", token.User)
	if token.Scope != "" {
		scopes := strings.ReplaceAll(token.Scope, " ", ", ")
		fmt.Fprintf(cmd.ErrOrStderr(), "  Scopes: %s\n", scopes)
	}
	return nil
}

func runAuthLogout(cmd *cobra.Command, _ []string) error {
	if err := config.DeleteAuth(); err != nil {
		return err
	}

	fmt.Fprintln(cmd.ErrOrStderr(), "✓ Logged out")
	return nil
}
