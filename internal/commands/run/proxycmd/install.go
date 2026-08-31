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
		Short: "Redirect port 80 to the run proxy so named URLs drop their port",
		Long:  "Write the OS files that redirect port 80 to the run proxy. The recap shows every file before sudo is asked for, and `wtm run proxy uninstall` reverses all of it.",
		RunE:  runInstall,
	}
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip the confirmation (sudo still asks for a password)")
	shared.AddOutputFlag(cmd)
	return cmd
}

func newUninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdUninstall,
		Short: "Remove the redirection and give named URLs their port back",
		RunE:  runUninstall,
	}
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip the confirmation (sudo still asks for a password)")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runInstall(cmd *cobra.Command, _ []string) error {
	bindPort, err := requireTerminal(cmd)
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
			Title:  domain.ProxyInstallRecapTitle,
			Files:  plan.Files,
			Script: plan.Script,
		})
	})

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
	if _, err := requireTerminal(cmd); err != nil {
		return err
	}

	redirector := proxy.NewRedirector(proxy.RedirectorParams{})
	if status := redirector.Inspect(); !status.Supported {
		return domain.ErrProxyRedirectUnsupported
	}

	output.Frame(cmd.OutOrStdout(), func() {
		output.ProxyPlanReport(cmd.OutOrStdout(), output.ProxyPlanReportParams{
			Title: domain.ProxyUninstallRecapTitle,
			Files: []domain.ProxyPlannedFile{
				{Path: domain.ProxyAnchorPath, Content: domain.ProxyUninstallRemoved},
				{Path: domain.ProxyPlistPath, Content: domain.ProxyUninstallRemoved},
				{Path: domain.ProxyPfConfPath, Content: domain.ProxyUninstallPfConfLine},
			},
		})
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

// requireTerminal refuses a privileged write nobody is watching and returns the
// bind port to redirect to.
func requireTerminal(cmd *cobra.Command) (int, error) {
	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	if format == domain.OutputJSON || !term.IsTerminal(int(os.Stdin.Fd())) {
		return 0, domain.ErrProxyInstallNeedsTTY
	}
	return configuredBindPort(cmd)
}
