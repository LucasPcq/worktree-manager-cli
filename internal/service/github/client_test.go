package github

import (
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestNeedsRefresh_EnvSource(t *testing.T) {
	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	auth := domain.AuthToken{
		AccessToken: "ghp_xxx",
		Source:      domain.AuthSourceEnv,
		ExpiresAt:   now.Add(-time.Hour), // already expired, should still be ignored
	}
	if needsRefresh(auth, now) {
		t.Error("env-sourced token should never need refresh")
	}
}

func TestNeedsRefresh_NoExpiration(t *testing.T) {
	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	auth := domain.AuthToken{
		AccessToken: "gho_xxx",
		Source:      domain.AuthSourceFile,
		// ExpiresAt is zero — non-expiring OAuth app
	}
	if needsRefresh(auth, now) {
		t.Error("token with zero ExpiresAt should not need refresh")
	}
}

func TestNeedsRefresh_NotYetExpired(t *testing.T) {
	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	auth := domain.AuthToken{
		AccessToken: "gho_xxx",
		Source:      domain.AuthSourceFile,
		ExpiresAt:   now.Add(2 * time.Hour),
	}
	if needsRefresh(auth, now) {
		t.Error("token valid for 2h should not need refresh")
	}
}

func TestNeedsRefresh_WithinBuffer(t *testing.T) {
	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	auth := domain.AuthToken{
		AccessToken: "gho_xxx",
		Source:      domain.AuthSourceFile,
		ExpiresAt:   now.Add(30 * time.Second), // within 60s buffer
	}
	if !needsRefresh(auth, now) {
		t.Error("token within refresh buffer should need refresh")
	}
}

func TestNeedsRefresh_Expired(t *testing.T) {
	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	auth := domain.AuthToken{
		AccessToken: "gho_xxx",
		Source:      domain.AuthSourceFile,
		ExpiresAt:   now.Add(-time.Hour),
	}
	if !needsRefresh(auth, now) {
		t.Error("expired token should need refresh")
	}
}
