package config

import (
	"fmt"

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
	config, err := config.GetBackplaneConfiguration()
	if err != nil {
		return err
	}

	proxyURL := ""
	if config.ProxyURL != nil {
		proxyURL = *config.ProxyURL
	}

	switch args[0] {
	case URLConfigVar:
		fmt.Printf("%s: %s\n", URLConfigVar, config.URL)
	case ProxyURLConfigVar:
		fmt.Printf("%s: %s\n", ProxyURLConfigVar, proxyURL)
	case SessionConfigVar:
		fmt.Printf("%s: %s\n", SessionConfigVar, config.SessionDirectory)
	case PagerDutyAPIConfigVar:
		fmt.Printf("%s: %s\n", PagerDutyAPIConfigVar, config.PagerDutyAPIKey)
	case GovcloudVar:
		fmt.Printf("%s: %t\n", GovcloudVar, config.Govcloud)
	case "all":
		fmt.Printf("%s: %s\n", URLConfigVar, config.URL)
		fmt.Printf("%s: %s\n", ProxyURLConfigVar, proxyURL)
		fmt.Printf("%s: %s\n", SessionConfigVar, config.SessionDirectory)
		fmt.Printf("%s: %s\n", PagerDutyAPIConfigVar, config.PagerDutyAPIKey)
		fmt.Printf("%s: %t\n", GovcloudVar, config.Govcloud)
	default:
		return fmt.Errorf("supported config variables are %s, %s, %s, %s, & %s", URLConfigVar, ProxyURLConfigVar, SessionConfigVar, PagerDutyAPIConfigVar, GovcloudVar)
	}

	return nil
}