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

	switch args[0] {
	case URLConfigVar:
		fmt.Printf("%s: %s\n", URLConfigVar, config.URL)
	case ProxyURLConfigVar:
		fmt.Printf("%s: %s\n", ProxyURLConfigVar, config.ProxyURL)
	case SessionConfigVar:
		fmt.Printf("%s: %s\n", SessionConfigVar, config.SessionDirectory)
	case "all":
		fmt.Printf("%s: %s\n", URLConfigVar, config.URL)
		fmt.Printf("%s: %s\n", ProxyURLConfigVar, config.ProxyURL)
		fmt.Printf("%s: %s\n", SessionConfigVar, config.SessionDirectory)
	default:
		return fmt.Errorf("supported config variables are %s, %s & %s", URLConfigVar, ProxyURLConfigVar, SessionConfigVar)
	}

	return nil
}
