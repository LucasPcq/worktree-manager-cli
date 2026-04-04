package github

import (
	"context"

	"github.com/google/go-github/v72/github"

	"github.com/LucasPcq/wtm/internal/config"
)

// NewClient creates an authenticated GitHub API client.
func NewClient(token string) *github.Client {
	return github.NewClient(nil).WithAuthToken(token)
}

// NewClientFromAuth loads the stored token and creates an authenticated client.
// Returns ErrAuthNotConfigured if no token is stored.
func NewClientFromAuth() (*github.Client, error) {
	auth, err := config.LoadAuth()
	if err != nil {
		return nil, err
	}
	return NewClient(auth.AccessToken), nil
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
