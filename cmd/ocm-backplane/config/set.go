package config

import (
	"fmt"
	"os"
	"path"

	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
	"gopkg.in/AlecAivazis/survey.v1"

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
	configPath, err := config.GetConfigFilePath()
	if err != nil {
		return err
	}

	if _, err = os.Stat(configPath); err == nil {
		viper.SetConfigFile(configPath)
		if err := viper.ReadInConfig(); err != nil {
			return err
		}
	}

	// create config directory if it doesn't exist
	if dir, err := os.Stat(path.Dir(configPath)); os.IsNotExist(err) || !dir.IsDir() {
		// check if stdout is a terminal. if so, prompt user to create config directory
		if term.IsTerminal(int(os.Stdout.Fd())) {
			confirm := false
			prompt := &survey.Confirm{
				Message: fmt.Sprintf("Config directory \"%s\" does not exist. Create it?", path.Dir(configPath)),
				Default: true,
			}
			if err := survey.AskOne(prompt, &confirm, nil); err != nil {
				return err
			}
			if confirm {
				if err := os.MkdirAll(path.Dir(configPath), 0750); err != nil {
					return err
				}
			} else {
				fmt.Println("Aborted")
				return nil
			}
		} else {
			// if we aren't in a terminal, just return an error
			return fmt.Errorf("config directory does not exist: %s", path.Dir(configPath))
		}
	}

	switch args[0] {
	case URLConfigVar:
		viper.Set(URLConfigVar, args[1])
	case ProxyURLConfigVar:
		viper.Set(ProxyURLConfigVar, args[1])
	case SessionConfigVar:
		viper.Set(SessionConfigVar, args[1])
	case PagerDutyAPIConfigVar:
		viper.Set(PagerDutyAPIConfigVar, args[1])
	case config.JiraTokenViperKey:
		viper.Set(config.JiraTokenViperKey, args[1])
	case config.JiraEmailViperKey:
		viper.Set(config.JiraEmailViperKey, args[1])
	case GovcloudVar:
		govcloud, err := strconv.ParseBool(args[1])
		if err != nil {
			return fmt.Errorf("invalid value for %s: %v", GovcloudVar, err)
		}
		viper.Set(GovcloudVar, govcloud)
	default:
		return fmt.Errorf("supported config variables are %s, %s, %s, %s, %s, %s & %s", URLConfigVar, ProxyURLConfigVar, SessionConfigVar, PagerDutyAPIConfigVar, config.JiraTokenViperKey, config.JiraEmailViperKey, GovcloudVar)
	}

	viper.SetConfigType("json")
	err = viper.WriteConfigAs(configPath)
	if err != nil {
		return err
	}
	fmt.Println("Configuration file updated at " + configPath)

	return nil
}