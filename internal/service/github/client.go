package github

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/go-github/v72/github"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
)

// tokenRefreshBuffer ensures we refresh slightly before true expiry to avoid
// racing with in-flight requests.
const tokenRefreshBuffer = 60 * time.Second

// NewClient creates an authenticated GitHub API client.
func NewClient(token string) *github.Client {
	return github.NewClient(nil).WithAuthToken(token)
}

// NewClientFromAuth resolves the stored token (refreshing if expired) and
// creates an authenticated client.
// Returns ErrAuthNotConfigured if no token is stored, or ErrAuthNeedsReauth
// if the token is expired and cannot be refreshed.
func NewClientFromAuth() (*github.Client, error) {
	auth, err := ResolveAuth()
	if err != nil {
		return nil, err
	}
	return NewClient(auth.AccessToken), nil
}

// ResolveAuth loads the stored token and refreshes it if expired.
// The refreshed token is persisted back to disk.
// Returns domain.ErrAuthNeedsReauth if the token is expired and no valid
// refresh token is available.
func ResolveAuth() (domain.AuthToken, error) {
	auth, err := config.LoadAuth()
	if err != nil {
		return domain.AuthToken{}, err
	}

	if !needsRefresh(auth, time.Now()) {
		return auth, nil
	}

	if auth.RefreshToken == "" {
		return domain.AuthToken{}, domain.ErrAuthNeedsReauth
	}
	if !auth.RefreshTokenExpiresAt.IsZero() && time.Now().After(auth.RefreshTokenExpiresAt) {
		return domain.AuthToken{}, domain.ErrAuthNeedsReauth
	}

	refreshed, err := refreshAccessToken(auth.RefreshToken)
	if err != nil {
		return domain.AuthToken{}, errors.Join(domain.ErrAuthNeedsReauth, err)
	}

	// Preserve user/source — not returned by the refresh endpoint
	refreshed.User = auth.User
	refreshed.Source = domain.AuthSourceFile

	if err := config.SaveAuth(refreshed); err != nil {
		return domain.AuthToken{}, fmt.Errorf("save refreshed token: %w", err)
	}

	return refreshed, nil
}

// needsRefresh reports whether the token must be refreshed before the next
// API call. Pure function — clock is injected so it can be unit-tested.
func needsRefresh(auth domain.AuthToken, now time.Time) bool {
	// PAT / env var — no refresh possible or desired
	if auth.Source == domain.AuthSourceEnv {
		return false
	}
	// OAuth App without expiration — treat as non-expiring
	if auth.ExpiresAt.IsZero() {
		return false
	}
	return !now.Add(tokenRefreshBuffer).Before(auth.ExpiresAt)
}

// VerifyToken checks that the stored token is still valid by calling the GitHub API.
func VerifyToken(token string) (string, error) {
	client := NewClient(token)
	user, _, err := client.Users.Get(context.Background(), "")
	if err != nil {
		return "", err
	}
	return user.GetLogin(), nil
}
