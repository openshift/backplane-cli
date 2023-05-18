package config

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/openshift/backplane-cli/pkg/cli/config"
)

func newSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "set",
		Short:        "Set Backplane CLI configuration variables",
		Example:      "ocm backplane config set url https://example.com",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(2),
		RunE:         setConfig,
	}

	return cmd
}

func setConfig(cmd *cobra.Command, args []string) error {
	bpConfig := &config.BackplaneConfiguration{}

	// Retrieve default Backplane CLI config path, $HOME/.config/backplane/config.json
	configPath, err := config.GetConfigFilePath()
	if err != nil {
		return err
	}

	if _, err = os.Stat(configPath); err == nil {
		viper.SetConfigFile(configPath)
		if err := viper.ReadInConfig(); err != nil {
			return err
		}

		bpConfig.URL = viper.GetString("url")
		bpConfig.ProxyURL = viper.GetString("proxy-url")
		bpConfig.SessionDirectory = viper.GetString("session-dir")
	}

	switch args[0] {
	case URLConfigVar:
		bpConfig.URL = args[1]
	case ProxyURLConfigVar:
		bpConfig.ProxyURL = args[1]
	case SessionConfigVar:
		bpConfig.SessionDirectory = args[1]
	default:
		return fmt.Errorf("supported config variables are %s, %s & %s", URLConfigVar, ProxyURLConfigVar, SessionConfigVar)
	}

	viper.SetConfigType("json")
	viper.Set(URLConfigVar, bpConfig.URL)
	viper.Set(ProxyURLConfigVar, bpConfig.ProxyURL)
	viper.Set(SessionConfigVar, bpConfig.SessionDirectory)

	err = viper.WriteConfigAs(configPath)
	if err != nil {
		return err
	}
	fmt.Println("Configuration file updated at " + configPath)

	return nil
}
