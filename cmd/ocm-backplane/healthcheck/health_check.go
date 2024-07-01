package healthcheck

import (
	"github.com/openshift/backplane-cli/pkg/healthcheck"
	"github.com/spf13/cobra"
)

var (
	checkVPN   bool
	checkProxy bool
)

// HealthCheckCmd is the command for performing health checks
var HealthCheckCmd = &cobra.Command{
	Use:     "healthcheck",
	Aliases: []string{"healthCheck", "health-check", "healthchecks"},
	Short:   "Check VPN and Proxy connectivity on the localhost",
	Run: func(cmd *cobra.Command, args []string) {
		healthcheck.RunHealthCheck(checkVPN, checkProxy)(cmd, args)
	},
}

func init() {
	HealthCheckCmd.Flags().BoolVar(&checkVPN, "vpn", false, "Check only VPN connectivity")
	HealthCheckCmd.Flags().BoolVar(&checkProxy, "proxy", false, "Check only Proxy connectivity")
}
