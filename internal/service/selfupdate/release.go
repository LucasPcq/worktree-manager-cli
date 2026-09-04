// Package selfupdate resolves how wtm was installed and brings it to the latest
// published release.
package selfupdate

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

const maxPayloadBytes = 1 << 20

type FetchReleaseParams struct {
	BaseURL   string
	Tag       string
	UserAgent string
	Timeout   time.Duration
}

type releasePayload struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func FetchRelease(params FetchReleaseParams) (domain.ReleaseInfo, error) {
	base := params.BaseURL
	if base == "" {
		base = domain.ReleaseAPIBase
	}

	url := base + "/latest"
	if params.Tag != "" {
		url = base + "/tags/v" + rules.NormalizeVersion(params.Tag)
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return domain.ReleaseInfo{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if params.UserAgent != "" {
		req.Header.Set("User-Agent", params.UserAgent)
	}

	client := &http.Client{Timeout: params.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return domain.ReleaseInfo{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return domain.ReleaseInfo{}, domain.ErrReleaseNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return domain.ReleaseInfo{}, fmt.Errorf("github releases API: %s", resp.Status)
	}

	var payload releasePayload
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxPayloadBytes)).Decode(&payload); err != nil {
		return domain.ReleaseInfo{}, fmt.Errorf("decode release: %w", err)
	}

	info := domain.ReleaseInfo{
		Version: rules.NormalizeVersion(payload.TagName),
		Tag:     payload.TagName,
		URL:     payload.HTMLURL,
	}
	for _, asset := range payload.Assets {
		info.Assets = append(info.Assets, domain.ReleaseAsset{Name: asset.Name, URL: asset.URL})
	}

	return info, nil
}
