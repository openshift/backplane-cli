package config

import (
	"fmt"
	"os"
	"path"

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
		proxyURL := viper.GetString("proxy-url")
		if proxyURL != "" {
			bpConfig.ProxyURL = &proxyURL
		}

		bpConfig.SessionDirectory = viper.GetString("session-dir")
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
		bpConfig.URL = args[1]
	case ProxyURLConfigVar:
		bpConfig.ProxyURL = &args[1]
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
