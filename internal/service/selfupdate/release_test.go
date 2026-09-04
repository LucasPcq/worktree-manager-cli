package selfupdate_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/selfupdate"
)

const releaseJSON = `{
  "tag_name": "v0.27.0",
  "html_url": "https://github.com/LucasPcq/wtm/releases/tag/v0.27.0",
  "assets": [
    {"name": "checksums.txt", "browser_download_url": "https://example.test/checksums.txt"},
    {"name": "wtm_0.27.0_darwin_arm64.tar.gz", "browser_download_url": "https://example.test/wtm.tar.gz"}
  ]
}`

func TestFetchReleaseLatest(t *testing.T) {
	var gotPath, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotUA = r.URL.Path, r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(releaseJSON))
	}))
	defer srv.Close()

	info, err := selfupdate.FetchRelease(selfupdate.FetchReleaseParams{
		BaseURL:   srv.URL,
		UserAgent: "wtm/0.26.1",
		Timeout:   2 * time.Second,
	})
	if err != nil {
		t.Fatalf("FetchRelease: %v", err)
	}

	if gotPath != "/latest" {
		t.Fatalf("path = %q, want /latest", gotPath)
	}
	if gotUA != "wtm/0.26.1" {
		t.Fatalf("user-agent = %q, want wtm/0.26.1", gotUA)
	}
	if info.Version != "0.27.0" {
		t.Fatalf("Version = %q, want 0.27.0 (leading v stripped)", info.Version)
	}
	if info.Tag != "v0.27.0" {
		t.Fatalf("Tag = %q, want v0.27.0", info.Tag)
	}
	if len(info.Assets) != 2 {
		t.Fatalf("Assets = %d, want 2", len(info.Assets))
	}
}

func TestFetchReleaseByTag(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(releaseJSON))
	}))
	defer srv.Close()

	if _, err := selfupdate.FetchRelease(selfupdate.FetchReleaseParams{
		BaseURL: srv.URL,
		Tag:     "0.25.0",
		Timeout: 2 * time.Second,
	}); err != nil {
		t.Fatalf("FetchRelease: %v", err)
	}

	if gotPath != "/tags/v0.25.0" {
		t.Fatalf("path = %q, want /tags/v0.25.0 (leading v added back)", gotPath)
	}
}

func TestFetchReleaseNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := selfupdate.FetchRelease(selfupdate.FetchReleaseParams{BaseURL: srv.URL, Timeout: 2 * time.Second})
	if !errors.Is(err, domain.ErrReleaseNotFound) {
		t.Fatalf("err = %v, want ErrReleaseNotFound", err)
	}
}

func TestFetchReleaseRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	if _, err := selfupdate.FetchRelease(selfupdate.FetchReleaseParams{BaseURL: srv.URL, Timeout: 2 * time.Second}); err == nil {
		t.Fatal("want an error on 403, got nil")
	}
}

func TestFetchReleaseMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	if _, err := selfupdate.FetchRelease(selfupdate.FetchReleaseParams{BaseURL: srv.URL, Timeout: 2 * time.Second}); err == nil {
		t.Fatal("want an error on malformed JSON, got nil")
	}
}
