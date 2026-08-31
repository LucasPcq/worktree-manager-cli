package proxycmd

import (
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/service/proxy"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdInstall,
		Short: "Serve named URLs on port 80 so they drop their port",
		Long:  "Install a per-user LaunchAgent: launchd binds port 80 on the loopback and hands the socket to wtm, which relays it to the run proxy. No sudo, no system file — everything lives in ~/Library/LaunchAgents and `wtm run proxy uninstall` removes it.",
		RunE:  runInstall,
	}
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip the confirmation")
	cmd.Flags().Bool(domain.FlagDryRun, false, "Print every file in full and write nothing")
	shared.AddOutputFlag(cmd)
	return cmd
}

func newUninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdUninstall,
		Short: "Remove the redirection and give named URLs their port back",
		RunE:  runUninstall,
	}
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip the confirmation")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runInstall(cmd *cobra.Command, _ []string) error {
	// --dry-run writes nothing, so it is the one path that needs no terminal.
	full, _ := cmd.Flags().GetBool(domain.FlagDryRun)
	bindPort, err := resolveBindPort(cmd, full)
	if err != nil {
		return err
	}

	redirector := proxy.NewRedirector(proxy.RedirectorParams{})
	plan, err := redirector.Plan(proxy.PlanParams{BindPort: bindPort})
	if err != nil {
		return err
	}

	output.Frame(cmd.OutOrStdout(), func() {
		output.ProxyPlanReport(cmd.OutOrStdout(), output.ProxyPlanReportParams{
			Files:      plan.Files,
			Script:     plan.Script,
			Full:       full,
			Reversible: true,
		})
	})
	if full {
		return nil
	}

	confirmed, err := confirm(cmd, components.NewConfirmParams{
		Title:       domain.ProxyInstallConfirmTitle,
		Description: domain.ProxyInstallConfirmDesc,
	})
	if err != nil || !confirmed {
		return err
	}

	if applyErr := redirector.Apply(proxy.PlanParams{BindPort: bindPort}); applyErr != nil {
		return applyErr
	}
	output.Frame(cmd.OutOrStdout(), func() {
		output.Success(cmd.OutOrStdout(), domain.ProxyInstallDone)
	})
	return nil
}

func runUninstall(cmd *cobra.Command, _ []string) error {
	if _, err := resolveBindPort(cmd, false); err != nil {
		return err
	}

	redirector := proxy.NewRedirector(proxy.RedirectorParams{})
	status := redirector.Inspect()
	if !status.Supported {
		return domain.ErrProxyRedirectUnsupported
	}

	plan, err := redirector.Plan(proxy.PlanParams{BindPort: status.BindPort})
	if err != nil {
		return err
	}
	for i := range plan.Files {
		plan.Files[i].Change = domain.ProxyUninstallChange
	}

	output.Frame(cmd.OutOrStdout(), func() {
		output.ProxyPlanReport(cmd.OutOrStdout(), output.ProxyPlanReportParams{Files: plan.Files})
	})

	confirmed, err := confirm(cmd, components.NewConfirmParams{
		Title:       domain.ProxyUninstallConfirmTitle,
		Description: domain.ProxyUninstallConfirmDesc,
	})
	if err != nil || !confirmed {
		return err
	}

	if removeErr := redirector.Remove(); removeErr != nil {
		return removeErr
	}
	output.Frame(cmd.OutOrStdout(), func() {
		output.Success(cmd.OutOrStdout(), domain.ProxyUninstallDone)
	})
	return nil
}

func confirm(cmd *cobra.Command, params components.NewConfirmParams) (bool, error) {
	if yes, _ := cmd.Flags().GetBool(domain.FlagYes); yes {
		return true, nil
	}
	confirmed, err := components.RunStandaloneConfirm(components.NewConfirm(params))
	return confirmed, err
}

// resolveBindPort returns the port to redirect to, refusing only what cannot be
// confirmed: launchd needs no privilege, so the terminal is only about the
// prompt, and --yes stands in for it.
func resolveBindPort(cmd *cobra.Command, preview bool) (int, error) {
	if !preview && !canConfirm(cmd) {
		return 0, domain.ErrProxyInstallNeedsYes
	}
	return configuredBindPort(cmd)
}

func canConfirm(cmd *cobra.Command) bool {
	if yes, _ := cmd.Flags().GetBool(domain.FlagYes); yes {
		return true
	}
	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	return format != domain.OutputJSON && term.IsTerminal(int(os.Stdin.Fd()))
}
