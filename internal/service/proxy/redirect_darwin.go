//go:build darwin

package proxy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/LucasPcq/wtm/internal/domain"
)

type launchdRedirector struct{ home string }

func NewRedirector(params RedirectorParams) Redirector {
	home := params.Root
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	return launchdRedirector{home: home}
}

func (r launchdRedirector) plistPath() string {
	return filepath.Join(r.home, domain.ProxyAgentDir, domain.ProxyPlistName)
}

func (r launchdRedirector) Plan(params PlanParams) (Plan, error) {
	exe, err := os.Executable()
	if err != nil {
		return Plan{}, fmt.Errorf("locate the wtm binary: %w", err)
	}

	path := r.plistPath()
	return Plan{
		Files: []domain.ProxyPlannedFile{{
			Path: path,
			Content: fmt.Sprintf(domain.ProxyPlistFmt,
				domain.ProxyPlistLabel, exe, domain.CmdProxyForward, domain.FlagTarget, params.BindPort,
				domain.ProxySocketKey, domain.ProxyPrivilegedPort, domain.ProxyPrivilegedPort),
			Change: fmt.Sprintf(domain.ProxyAgentChangeFmt, domain.ProxyPrivilegedPort, params.BindPort),
		}},
		Script: fmt.Sprintf(domain.ProxyLoadCmdFmt, path),
	}, nil
}

func (r launchdRedirector) Apply(params PlanParams) error {
	plan, err := r.Plan(params)
	if err != nil {
		return err
	}

	path := r.plistPath()
	if mkErr := os.MkdirAll(filepath.Dir(path), 0o755); mkErr != nil {
		return mkErr
	}
	// A reinstall onto a loaded agent would keep serving the old target, so the
	// unload comes first and its failure is the expected case, not an error.
	_ = exec.Command(domain.LaunchctlBin, domain.LaunchctlUnload, path).Run()
	if writeErr := os.WriteFile(path, []byte(plan.Files[0].Content), 0o644); writeErr != nil {
		return writeErr
	}

	out, err := exec.Command(domain.LaunchctlBin, domain.LaunchctlLoad, path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl load: %w: %s", err, out)
	}
	return nil
}

func (r launchdRedirector) Remove() error {
	path := r.plistPath()
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	_ = exec.Command(domain.LaunchctlBin, domain.LaunchctlUnload, path).Run()
	return os.Remove(path)
}

var agentTargetPort = regexp.MustCompile(`--target</string>\s*<string>(\d+)</string>`)

func (r launchdRedirector) Inspect() domain.ProxyStatus {
	status := domain.ProxyStatus{Supported: true, Mechanism: domain.ProxyMechanismLaunchd}

	plist, err := os.ReadFile(r.plistPath())
	if err != nil {
		return status
	}

	status.Installed = true
	if match := agentTargetPort.FindStringSubmatch(string(plist)); len(match) == 2 {
		status.BindPort, _ = strconv.Atoi(match[1])
	}
	return status
}
