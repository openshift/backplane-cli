package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

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
	configPath, err := config.GetConfigFilePath()
	if err != nil {
		return err
	}

	if _, err = os.Stat(configPath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("unable to access config file %s: %w", configPath, err)
		}
	} else {
		viper.SetConfigFile(configPath)
		viper.SetConfigType("json")
		if err := viper.ReadInConfig(); err != nil {
			return err
		}
	}

	switch args[0] {
	case URLConfigVar:
		fmt.Printf("%s: %s\n", URLConfigVar, viper.GetString(URLConfigVar))
	case ProxyURLConfigVar:
		fmt.Printf("%s: %s\n", ProxyURLConfigVar, strings.Join(viper.GetStringSlice(ProxyURLConfigVar), ", "))
	case SessionConfigVar:
		fmt.Printf("%s: %s\n", SessionConfigVar, viper.GetString(SessionConfigVar))
	case PagerDutyAPIConfigVar:
		fmt.Printf("%s: %s\n", PagerDutyAPIConfigVar, viper.GetString(PagerDutyAPIConfigVar))
	case GovcloudVar:
		fmt.Printf("%s: %t\n", GovcloudVar, viper.GetBool(GovcloudVar))
	case "all":
		fmt.Printf("%s: %s\n", URLConfigVar, viper.GetString(URLConfigVar))
		fmt.Printf("%s: %s\n", ProxyURLConfigVar, strings.Join(viper.GetStringSlice(ProxyURLConfigVar), ", "))
		fmt.Printf("%s: %s\n", SessionConfigVar, viper.GetString(SessionConfigVar))
		fmt.Printf("%s: %s\n", PagerDutyAPIConfigVar, viper.GetString(PagerDutyAPIConfigVar))
		fmt.Printf("%s: %t\n", GovcloudVar, viper.GetBool(GovcloudVar))
	default:
		return fmt.Errorf("supported config variables are %s, %s, %s, %s, & %s", URLConfigVar, ProxyURLConfigVar, SessionConfigVar, PagerDutyAPIConfigVar, GovcloudVar)
	}

	return nil
}