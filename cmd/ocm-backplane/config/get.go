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
		Args:         cobra.MinimumNArgs(1),
		RunE:         getConfig,
	}
	return cmd
}

func getConfig(cmd *cobra.Command, args []string) error {
	config, err := config.GetBackplaneConfiguration()
	if err != nil {
		return err
	}

	switch args[0] {
	case URLConfigVar:
		fmt.Printf("%s: %s\n", URLConfigVar, config.URL)
	case ProxyURLConfigVar:
		fmt.Printf("%s: %s\n", ProxyURLConfigVar, config.ProxyURL)
	case "all":
		fmt.Printf("%s: %s\n", URLConfigVar, config.URL)
		fmt.Printf("%s: %s\n", ProxyURLConfigVar, config.ProxyURL)
	default:
		return fmt.Errorf("supported config variables are %s and %s", URLConfigVar, ProxyURLConfigVar)
	}

	return nil
}
