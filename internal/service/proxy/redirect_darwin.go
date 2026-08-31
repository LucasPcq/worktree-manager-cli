//go:build darwin

package proxy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type pfRedirector struct{ root string }

func NewRedirector(params RedirectorParams) Redirector {
	root := params.Root
	if root == "" {
		root = "/"
	}
	return pfRedirector{root: root}
}

func (r pfRedirector) at(path string) string { return filepath.Join(r.root, path) }

func (r pfRedirector) Plan(params PlanParams) (Plan, error) {
	current, err := os.ReadFile(r.at(domain.ProxyPfConfPath))
	if err != nil {
		return Plan{}, fmt.Errorf("read %s: %w", domain.ProxyPfConfPath, err)
	}
	conf, err := rules.PfConfWithWTM(string(current))
	if err != nil {
		return Plan{}, err
	}

	plan := Plan{Files: []domain.ProxyPlannedFile{
		{
			Path:    domain.ProxyAnchorPath,
			Content: fmt.Sprintf(domain.ProxyAnchorRuleFmt, domain.ProxyPrivilegedPort, params.BindPort),
			Change:  fmt.Sprintf(domain.ProxyAnchorChangeFmt, domain.ProxyPrivilegedPort, params.BindPort),
		},
		{Path: domain.ProxyPfConfPath, Content: conf, Change: domain.ProxyPfConfChange},
		{
			Path:    domain.ProxyPlistPath,
			Content: fmt.Sprintf(domain.ProxyPlistFmt, domain.ProxyPlistLabel, domain.ProxyPfConfPath),
			Change:  domain.ProxyPlistChange,
		},
	}}
	plan.Script = installScript(plan)
	return plan, nil
}

func (r pfRedirector) Apply(params PlanParams) error {
	plan, err := r.Plan(params)
	if err != nil {
		return err
	}

	staged, err := stage(plan)
	if err != nil {
		return err
	}
	defer os.RemoveAll(staged)

	return runPrivileged(staged, plan.Script)
}

func (r pfRedirector) Remove() error {
	conf, err := os.ReadFile(r.at(domain.ProxyPfConfPath))
	if err != nil {
		return fmt.Errorf("read %s: %w", domain.ProxyPfConfPath, err)
	}

	staged, err := stage(Plan{Files: []domain.ProxyPlannedFile{
		{Path: domain.ProxyPfConfPath, Content: rules.PfConfWithoutWTM(string(conf))},
	}})
	if err != nil {
		return err
	}
	defer os.RemoveAll(staged)

	return runPrivileged(staged, removeScript())
}

var anchorBindPort = regexp.MustCompile(`port (\d+)\s*$`)

func (r pfRedirector) Inspect() domain.ProxyStatus {
	status := domain.ProxyStatus{Supported: true, Mechanism: domain.ProxyMechanismPf}

	conf, err := os.ReadFile(r.at(domain.ProxyPfConfPath))
	if err != nil || !rules.PfConfHasWTM(string(conf)) {
		return status
	}
	if _, statErr := os.Stat(r.at(domain.ProxyPlistPath)); statErr != nil {
		return status
	}
	anchor, err := os.ReadFile(r.at(domain.ProxyAnchorPath))
	if err != nil {
		return status
	}

	status.Installed = true
	if match := anchorBindPort.FindStringSubmatch(strings.TrimSpace(string(anchor))); len(match) == 2 {
		status.BindPort, _ = strconv.Atoi(match[1])
	}
	return status
}

// stage writes the plan into an unprivileged temp dir under flat names; the
// script installs from there, so nothing privileged is ever handed a path the
// caller could still be writing to.
func stage(plan Plan) (string, error) {
	dir, err := os.MkdirTemp("", "wtm-proxy-")
	if err != nil {
		return "", err
	}
	for _, file := range plan.Files {
		if writeErr := os.WriteFile(filepath.Join(dir, filepath.Base(file.Path)), []byte(file.Content), 0o644); writeErr != nil {
			os.RemoveAll(dir)
			return "", writeErr
		}
	}
	return dir, nil
}

// runPrivileged asks once, not per file: a password prompt per step is how a
// user stops reading what they are approving.
func runPrivileged(dir, script string) error {
	cmd := exec.Command("sudo", "/bin/sh", "-c", script)
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func installScript(plan Plan) string {
	lines := []string{"set -e"}
	for _, file := range plan.Files {
		lines = append(lines, fmt.Sprintf("install -m 0644 %q %q", filepath.Base(file.Path), file.Path))
	}
	return strings.Join(append(lines,
		fmt.Sprintf("launchctl bootstrap system %q || true", domain.ProxyPlistPath),
		fmt.Sprintf("pfctl -E -f %q", domain.ProxyPfConfPath),
	), "\n")
}

func removeScript() string {
	return strings.Join([]string{
		"set -e",
		fmt.Sprintf("install -m 0644 %q %q", filepath.Base(domain.ProxyPfConfPath), domain.ProxyPfConfPath),
		fmt.Sprintf("launchctl bootout system %q || true", domain.ProxyPlistPath),
		fmt.Sprintf("rm -f %q %q", domain.ProxyPlistPath, domain.ProxyAnchorPath),
		fmt.Sprintf("pfctl -f %q", domain.ProxyPfConfPath),
	}, "\n")
}
