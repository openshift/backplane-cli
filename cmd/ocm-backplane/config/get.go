package config

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/openshift/backplane-cli/pkg/cli/config"
)

func newGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "get",
		Short:        "Get Backplane CLI configuration variables",
		Example:      "ocm backplane config get url",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE:         getConfig,
	}
	return cmd
}

func getConfig(cmd *cobra.Command, args []string) error {
	bpConfig, err := config.GetBackplaneConfiguration()
	if err != nil {
		return err
	}

	proxyURL := ""
	if bpConfig.ProxyURL != nil {
		proxyURL = *bpConfig.ProxyURL
	}

	switch args[0] {
	case URLConfigVar:
		fmt.Printf("%s: %s\n", URLConfigVar, bpConfig.URL)
	case ProxyURLConfigVar:
		fmt.Printf("%s: %s\n", ProxyURLConfigVar, proxyURL)
	case SessionConfigVar:
		fmt.Printf("%s: %s\n", SessionConfigVar, bpConfig.SessionDirectory)
	case PagerDutyAPIConfigVar:
		fmt.Printf("%s: %s\n", PagerDutyAPIConfigVar, bpConfig.PagerDutyAPIKey)
	case config.PluginViperKey:
		if len(bpConfig.PluginList) == 0 {
			fmt.Println("No plugins configured")
		} else {
			fmt.Printf("%s: [%s]\n", config.PluginViperKey, strings.Join(bpConfig.PluginList, ","))
		}
	case "all":
		fmt.Printf("%s: %s\n", URLConfigVar, bpConfig.URL)
		fmt.Printf("%s: %s\n", ProxyURLConfigVar, proxyURL)
		fmt.Printf("%s: %s\n", SessionConfigVar, bpConfig.SessionDirectory)
		fmt.Printf("%s: %s\n", PagerDutyAPIConfigVar, bpConfig.PagerDutyAPIKey)
		if len(bpConfig.PluginList) == 0 {
			fmt.Println("No plugins configured")
		} else {
			fmt.Printf("%s: [%s]\n", config.PluginViperKey, strings.Join(bpConfig.PluginList, ","))
		}
	default:
		return fmt.Errorf("supported config variables are %s, %s, %s, %s & %s", URLConfigVar, ProxyURLConfigVar, SessionConfigVar, PagerDutyAPIConfigVar, config.PluginViperKey)
	}

	return nil
}
