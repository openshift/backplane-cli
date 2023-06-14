package monitoring

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/openshift/backplane-cli/pkg/monitoring"
	"github.com/spf13/cobra"
)

var MonitoringCmd = &cobra.Command{
	Use:          fmt.Sprintf("monitoring <%s>", strings.Join(monitoring.ValidMonitoringNames, "|")),
	Short:        "Create a local proxy to the monitoring UI",
	Long:         fmt.Sprintf(`It will proxy to the monitoring UI including %s.`, strings.Join(monitoring.ValidMonitoringNames, ",")),
	Args:         cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	ValidArgs:    monitoring.ValidMonitoringNames,
	RunE:         runMonitoring,
	SilenceUsage: true,
}

func init() {
	flags := MonitoringCmd.Flags()
	flags.BoolVarP(
		&monitoring.MonitoringOpts.Browser,
		"browser",
		"b",
		false,
		"Open the browser automatically.",
	)
	flags.StringVarP(
		&monitoring.MonitoringOpts.Namespace,
		"namespace",
		"n",
		"openshift-monitoring",
		"Specify namespace of monitoring stack.",
	)
	flags.StringVarP(
		&monitoring.MonitoringOpts.Selector,
		"selector",
		"l",
		"",
		"Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2).",
	)
	flags.StringVarP(
		&monitoring.MonitoringOpts.Port,
		"port",
		"p",
		"",
		"The port the remote application listens on. (Default will be picked by server based on application's conventional port.)",
	)
	flags.StringVarP(
		&monitoring.MonitoringOpts.OriginUrl,
		"origin",
		"u",
		"",
		"The original url. Eg, copied from the prometheus url in pagerduty. When specified, it will print the proxied url of the corresponding original url.",
	)
	flags.StringVar(
		&monitoring.MonitoringOpts.ListenAddr,
		"listen",
		"",
		"The local address to listen to. Recommend using 127.0.0.1:xxxx to minimize security risk. The default will pick a random port on 127.0.0.1",
	)

}

// runMonitoring create local proxy url to serve monitoring dashboard
func runMonitoring(cmd *cobra.Command, argv []string) error {
	monitoringType := argv[0]
	monitoring.MonitoringOpts.KeepAlive = true
	client := monitoring.NewClient("", http.Client{})
	return client.RunMonitoring(monitoringType)
}
