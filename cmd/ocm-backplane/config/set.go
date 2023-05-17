package config

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/openshift/backplane-cli/pkg/cli/config"
)

var configFlags struct {
	url      string
	proxyURL string
}

func newSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "set",
		Short:        "Set Backplane CLI configuration variables",
		Example:      "ocm backplane config set --proxy-url <proxy> --url <url>",
		SilenceUsage: true,
		RunE:         setConfig,
	}

	cmd.Flags().StringVar(
		&configFlags.proxyURL,
		ProxyURLConfigVar,
		"",
		"Squid proxy URL",
	)

	cmd.Flags().StringVar(
		&configFlags.url,
		URLConfigVar,
		"",
		"Backplane API URL",
	)

	return cmd
}

func setConfig(cmd *cobra.Command, args []string) error {
	// Retrieve default Backplane CLI config path, $HOME/.config/backplane/config.json
	configPath, err := config.GetConfigFilePath()
	if err != nil {
		return err
	}

	if configFlags.proxyURL == "" {
		return fmt.Errorf("%s should not be empty", ProxyURLConfigVar)
	}

	if configFlags.url == "" {
		return fmt.Errorf("%s should not be empty", URLConfigVar)
	}

	viper.Set(ProxyURLConfigVar, configFlags.proxyURL)
	viper.Set(URLConfigVar, configFlags.url)

	err = viper.WriteConfigAs(configPath)
	if err != nil {
		return err
	}
	fmt.Println("Configuration file created at " + configPath)

	return nil
}
