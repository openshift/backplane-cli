package config

import (
	"github.com/spf13/cobra"
)

const (
	ProxyURLConfigVar     = "proxy-url"
	URLConfigVar          = "url"
	SessionConfigVar      = "session-dir"
	PagerDutyAPIConfigVar = "pd-key"
)

func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Get or set backplane-cli configuration",
		Long: `Get or set backplane-cli configuration variables.
The location of the configuration file is gleaned from ~/.config/backplane/config.json or the 'BACKPLANE_CONFIG' environment variable if set.

The following variables are supported:
url         Backplane API URL
proxy-url   Squid proxy URL
session-dir Backplane CLI session directory
pd-key      PagerDuty API User Key
`,
		SilenceUsage: true,
	}

	cmd.AddCommand(newGetCmd())
	cmd.AddCommand(newSetCmd())
	return cmd
}
