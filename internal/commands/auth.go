package commands

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	ghservice "github.com/LucasPcq/wtm/internal/service/github"
)

// NewAuthCmd creates the wtm auth command group.
func NewAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "auth",
		Short:   "Manage GitHub authentication",
		GroupID: domain.CmdGroupCore,
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
	if os.Getenv(domain.EnvGithubToken) != "" {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"%s is set — it takes priority over any stored token.\n"+
				"Unset it first, or run `wtm auth status` to confirm the active source.\n",
			domain.EnvGithubToken)
		return nil
	}

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

	switch token.Source {
	case domain.AuthSourceEnv:
		fmt.Fprintf(cmd.ErrOrStderr(), "✓ Authenticated via %s (env var)\n", domain.EnvGithubToken)
		if _, fileErr := authFileExists(); fileErr == nil {
			fmt.Fprintln(cmd.ErrOrStderr(), "  Note: a stored token exists but is shadowed by the env var.")
		}
	case domain.AuthSourceFile:
		fmt.Fprintf(cmd.ErrOrStderr(), "✓ Authenticated as %s (via auth.json)\n", token.User)
		if token.Scope != "" {
			scopes := strings.ReplaceAll(token.Scope, " ", ", ")
			fmt.Fprintf(cmd.ErrOrStderr(), "  Scopes: %s\n", scopes)
		}
		printExpirationStatus(cmd, token)
	}
	return nil
}

// printExpirationStatus writes access/refresh token expiration info to stderr.
// When ExpiresAt is zero (OAuth App without expiration) nothing is printed.
func printExpirationStatus(cmd *cobra.Command, token domain.AuthToken) {
	if token.ExpiresAt.IsZero() {
		return
	}

	remaining := time.Until(token.ExpiresAt)
	switch {
	case remaining <= 0:
		fmt.Fprintln(cmd.ErrOrStderr(), "  Access token: expired (will refresh on next API call)")
	case remaining < time.Hour:
		fmt.Fprintf(cmd.ErrOrStderr(), "  Access token expires in %d min\n", int(remaining.Minutes()))
	default:
		fmt.Fprintf(cmd.ErrOrStderr(), "  Access token expires in %s\n", remaining.Truncate(time.Minute))
	}

	if !token.RefreshTokenExpiresAt.IsZero() && time.Now().After(token.RefreshTokenExpiresAt) {
		fmt.Fprintln(cmd.ErrOrStderr(), "  Refresh token: expired — run `wtm auth login` again")
	}
}

// authFileExists reports whether a stored auth.json is present on disk.
// Used by `auth status` to notify when an env var shadows a stored token.
func authFileExists() (bool, error) {
	path, err := config.AuthPath()
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(path); err != nil {
		return false, err
	}
	return true, nil
}

func runAuthLogout(cmd *cobra.Command, _ []string) error {
	if err := config.DeleteAuth(); err != nil {
		return err
	}

	fmt.Fprintln(cmd.ErrOrStderr(), "✓ Logged out")

	if os.Getenv(domain.EnvGithubToken) != "" {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"Note: %s is still set — wtm will continue using it until you unset it.\n",
			domain.EnvGithubToken)
	}
	return nil
}
