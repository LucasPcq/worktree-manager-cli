package daemon

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/proxy"
)

// NewProxyForwardCmd creates the hidden command launchd runs. It is not a user
// surface: launchd owns the privileged socket and hands it over, which is the
// whole reason wtm never needs a privilege of its own.
func NewProxyForwardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    domain.CmdProxyForward,
		Short:  "Relay the launchd-owned privileged socket to the run proxy (internal)",
		Hidden: true,
		RunE:   runProxyForward,
	}
	cmd.Flags().Int(domain.FlagTarget, domain.ProxyDefaultPort, "Port the run proxy listens on")
	return cmd
}

func runProxyForward(cmd *cobra.Command, _ []string) error {
	listeners, err := proxy.LaunchdListeners(domain.ProxySocketKey)
	if err != nil {
		return err
	}

	target, _ := cmd.Flags().GetInt(domain.FlagTarget)
	return proxy.Forward(proxy.ForwardParams{Listeners: listeners, Target: target})
}
