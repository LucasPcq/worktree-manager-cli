package domain

import "time"

type InstallMethod string

const (
	InstallHomebrew   InstallMethod = "homebrew"
	InstallGoInstall  InstallMethod = "go-install"
	InstallStandalone InstallMethod = "standalone"
	InstallSource     InstallMethod = "source"
)

type UpgradeAction string

const (
	UpgradeActionReplaced  UpgradeAction = "replaced"
	UpgradeActionDelegated UpgradeAction = "delegated"
	UpgradeActionNone      UpgradeAction = "none"
	UpgradeActionChecked   UpgradeAction = "checked"
)

type ReleaseAsset struct {
	Name string
	URL  string
}

type ReleaseInfo struct {
	Version string
	Tag     string
	URL     string
	Assets  []ReleaseAsset
}

type UpdateState struct {
	CheckedAt     time.Time `json:"checked_at"`
	LatestVersion string    `json:"latest_version"`
}

type UpgradeResult struct {
	Installed string        `json:"installed"`
	Latest    string        `json:"latest"`
	UpToDate  bool          `json:"up_to_date"`
	Method    InstallMethod `json:"method"`
	Action    UpgradeAction `json:"action"`
}
