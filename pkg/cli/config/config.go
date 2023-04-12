package config

import (
	"os"
	"path/filepath"

	"github.com/openshift/backplane-cli/pkg/info"
	"github.com/spf13/viper"
)

type BackplaneConfiguration struct {
	URL      string
	ProxyURL string
}

// getConfigFilePath returns the default config path
func getConfigFilePath() (string, error) {
	UserHomeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	configFilePath := filepath.Join(UserHomeDir, info.BACKPLANE_CONFIG_DEFAULT_FILE_PATH, info.BACKPLANE_CONFIG_DEFAULT_FILE_NAME)

	return configFilePath, nil
}

// GetBackplaneConfiguration parses and returns the given backplane configuration
func GetBackplaneConfiguration() (bpConfig BackplaneConfiguration, err error) {
	var filepath string

	// Check if user has explicitly defined backplane config path
	path, bpConfigFound := os.LookupEnv(info.BACKPLANE_CONFIG_PATH_ENV_NAME)

	if bpConfigFound {
		filepath = path
	} else {
		filepath, err = getConfigFilePath()
		if err != nil {
			return bpConfig, err
		}
	}

	viper.AutomaticEnv()

	// Check if the config file exists
	if _, err = os.Stat(filepath); err == nil {
		// Load config file
		viper.SetConfigFile(filepath)
		viper.SetConfigType("json")

		if err := viper.ReadInConfig(); err != nil {
			return bpConfig, err
		}
	}

	// Check if user has explicitly defined backplane URL; it has higher precedence over the config file
	err = viper.BindEnv("url", info.BACKPLANE_URL_ENV_NAME)
	if err != nil {
		return bpConfig, err
	}

	// Check if user has explicitly defined proxy; it has higher precedence over the config file
	err = viper.BindEnv("proxy-url", info.BACKPLANE_PROXY_ENV_NAME)
	if err != nil {
		return bpConfig, err
	}

	bpConfig.URL = viper.GetString("url")
	bpConfig.ProxyURL = viper.GetString("proxy-url")

	return bpConfig, nil
}
