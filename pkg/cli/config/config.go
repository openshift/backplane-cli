package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"

	"github.com/openshift/backplane-cli/pkg/info"
)

type BackplaneConfiguration struct {
	URL              string
	ProxyURL         string
	SessionDirectory string
}

// GetConfigFilePath returns the Backplane CLI configuration filepath
func GetConfigFilePath() (string, error) {
	// Check if user has explicitly defined backplane config path
	path, found := os.LookupEnv(info.BACKPLANE_CONFIG_PATH_ENV_NAME)
	if found {
		return path, nil
	}

	UserHomeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	configFilePath := filepath.Join(UserHomeDir, info.BACKPLANE_CONFIG_DEFAULT_FILE_PATH, info.BACKPLANE_CONFIG_DEFAULT_FILE_NAME)

	return configFilePath, nil
}

// GetBackplaneConfiguration parses and returns the given backplane configuration
func GetBackplaneConfiguration() (bpConfig BackplaneConfiguration, err error) {
	filePath, err := GetConfigFilePath()
	if err != nil {
		return bpConfig, err
	}

	viper.AutomaticEnv()

	// Check if the config file exists
	if _, err = os.Stat(filePath); err == nil {
		// Load config file
		viper.SetConfigFile(filePath)
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
	bpConfig.SessionDirectory = viper.GetString("session-dir")

	return bpConfig, nil
}
