// Package github provides GitHub API integration via OAuth Device Flow and go-github.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

// deviceCodeResponse is the response from the device/code endpoint.
type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// tokenResponse is the response from the token endpoint.
// ExpiresIn, RefreshToken and RefreshTokenExpiresIn are only populated when
// the GitHub OAuth App has token expiration enabled.
type tokenResponse struct {
	AccessToken           string `json:"access_token"`
	TokenType             string `json:"token_type"`
	Scope                 string `json:"scope"`
	ExpiresIn             int    `json:"expires_in"`
	RefreshToken          string `json:"refresh_token"`
	RefreshTokenExpiresIn int    `json:"refresh_token_expires_in"`
	Error                 string `json:"error"`
}

// DeviceFlowLoginParams holds inputs for the device flow login.
type DeviceFlowLoginParams struct {
	Output io.Writer
}

// DeviceFlowLogin runs the GitHub OAuth Device Flow and returns a token.
func DeviceFlowLogin(params DeviceFlowLoginParams) (domain.AuthToken, error) {
	deviceCode, err := requestDeviceCode()
	if err != nil {
		return domain.AuthToken{}, fmt.Errorf("request device code: %w", err)
	}

	fmt.Fprintf(params.Output, "Authenticate wtm with GitHub.\n")
	fmt.Fprintf(params.Output, "Visit: %s\n", deviceCode.VerificationURI)
	fmt.Fprintf(params.Output, "Code:  %s\n\n", deviceCode.UserCode)

	tokenResp, err := pollForToken(pollParams{
		DeviceCode: deviceCode.DeviceCode,
		Interval:   deviceCode.Interval,
		ExpiresIn:  deviceCode.ExpiresIn,
		Output:     params.Output,
	})
	if err != nil {
		return domain.AuthToken{}, err
	}

	username, err := fetchUsername(tokenResp.AccessToken)
	if err != nil {
		return domain.AuthToken{}, fmt.Errorf("fetch user: %w", err)
	}

	return buildAuthToken(tokenResp, username, time.Now()), nil
}

// buildAuthToken converts a tokenResponse to an AuthToken, populating
// ExpiresAt / RefreshTokenExpiresAt relative to the given time.
// When the OAuth App has no expiration, ExpiresIn is zero and the
// corresponding timestamp remains zero.
func buildAuthToken(resp tokenResponse, username string, now time.Time) domain.AuthToken {
	auth := domain.AuthToken{
		AccessToken:  resp.AccessToken,
		TokenType:    resp.TokenType,
		Scope:        resp.Scope,
		User:         username,
		RefreshToken: resp.RefreshToken,
	}
	if resp.ExpiresIn > 0 {
		auth.ExpiresAt = now.Add(time.Duration(resp.ExpiresIn) * time.Second)
	}
	if resp.RefreshTokenExpiresIn > 0 {
		auth.RefreshTokenExpiresAt = now.Add(time.Duration(resp.RefreshTokenExpiresIn) * time.Second)
	}
	return auth
}

func requestDeviceCode() (deviceCodeResponse, error) {
	form := url.Values{
		"client_id": {domain.GitHubClientID},
		"scope":     {domain.GitHubScopes},
	}

	req, err := http.NewRequest(http.MethodPost, domain.GitHubDeviceCodeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return deviceCodeResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return deviceCodeResponse{}, err
	}
	defer resp.Body.Close()

	var result deviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return deviceCodeResponse{}, fmt.Errorf("decode response: %w", err)
	}

	return result, nil
}

type pollParams struct {
	DeviceCode string
	Interval   int
	ExpiresIn  int
	Output     io.Writer
}

func pollForToken(params pollParams) (tokenResponse, error) {
	interval := time.Duration(params.Interval) * time.Second
	deadline := time.Now().Add(time.Duration(params.ExpiresIn) * time.Second)

	spinFrames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	frameIdx := 0

	for time.Now().Before(deadline) {
		fmt.Fprintf(params.Output, "\r%s Waiting for authorization...", spinFrames[frameIdx])
		frameIdx = (frameIdx + 1) % len(spinFrames)

		time.Sleep(interval)

		resp, err := requestToken(params.DeviceCode)
		if err != nil {
			return tokenResponse{}, err
		}

		switch resp.Error {
		case "":
			fmt.Fprintf(params.Output, "\r                                \r")
			return resp, nil
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5 * time.Second
			continue
		case "expired_token":
			fmt.Fprintf(params.Output, "\r                                \r")
			return tokenResponse{}, domain.ErrAuthDeviceFlowTimeout
		case "access_denied":
			fmt.Fprintf(params.Output, "\r                                \r")
			return tokenResponse{}, fmt.Errorf("access denied by user")
		default:
			fmt.Fprintf(params.Output, "\r                                \r")
			return tokenResponse{}, fmt.Errorf("unexpected error: %s", resp.Error)
		}
	}

	fmt.Fprintf(params.Output, "\r                                \r")
	return tokenResponse{}, domain.ErrAuthDeviceFlowTimeout
}

func requestToken(deviceCode string) (tokenResponse, error) {
	form := url.Values{
		"client_id":   {domain.GitHubClientID},
		"device_code": {deviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}

	req, err := http.NewRequest(http.MethodPost, domain.GitHubTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return tokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return tokenResponse{}, err
	}
	defer resp.Body.Close()

	var result tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return tokenResponse{}, fmt.Errorf("decode response: %w", err)
	}

	return result, nil
}

func fetchUsername(token string) (string, error) {
	client := NewClient(token)
	user, _, err := client.Users.Get(context.Background(), "")
	if err != nil {
		return "", err
	}
	return user.GetLogin(), nil
}

// refreshAccessToken exchanges a refresh token for a new access token via
// the GitHub OAuth token endpoint. The returned AuthToken does not carry
// the username or source — the caller must repopulate those fields.
func refreshAccessToken(refreshToken string) (domain.AuthToken, error) {
	form := url.Values{
		"client_id":     {domain.GitHubClientID},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}

	req, err := http.NewRequest(http.MethodPost, domain.GitHubTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return domain.AuthToken{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return domain.AuthToken{}, err
	}
	defer resp.Body.Close()

	var result tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return domain.AuthToken{}, fmt.Errorf("decode response: %w", err)
	}

	if result.Error != "" {
		return domain.AuthToken{}, fmt.Errorf("refresh failed: %s", result.Error)
	}

	return buildAuthToken(result, "", time.Now()), nil
}
