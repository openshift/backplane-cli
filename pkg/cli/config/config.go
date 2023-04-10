package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/openshift-online/ocm-cli/pkg/ocm"
	"github.com/openshift/backplane-cli/pkg/info"
	logger "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type BackplaneConfiguration struct {
	URL      string
	ProxyURL string `mapstructure:"proxy-url"`
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

// getOCMUrl returns the current OCM environment URL
func getOCMUrl() (string, error) {
	connection, err := ocm.NewConnection().Build()
	if err != nil {
		return "", fmt.Errorf("failed to create OCM connection: %v", err)
	}
	defer connection.Close()

	return connection.URL(), nil
}

// getEnvConfig fetches environment variables if defined
func getEnvConfig() (bpConfig BackplaneConfiguration, err error) {
	// Check if user has explicitly defined backplane URL
	bpURL, hasURL := os.LookupEnv(info.BACKPLANE_URL_ENV_NAME)

	if hasURL {
		if bpURL == "" {
			return bpConfig, fmt.Errorf("%s environment variable is empty", info.BACKPLANE_URL_ENV_NAME)
		}
		bpConfig.URL = bpURL
	}

	// Check if user has explicitly defined proxy
	proxyUrl, hasEnvProxyURL := os.LookupEnv(info.BACKPLANE_PROXY_ENV_NAME)

	if hasEnvProxyURL {
		// We do not check if the proxy URL has been specified here
		// The error handling on an empty proxy is done during login due to client proxy
		bpConfig.ProxyURL = proxyUrl
	}

	return bpConfig, nil
}

// GetBackplaneConfiguration reads configuration from the provided config file
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

	// Check if the config file exists
	// If not, look for any defined env variables
	if _, err = os.Stat(filepath); err != nil {
		logger.Warnf(
			"Configuration file not found, searching for environment variables: %s, %s",
			info.BACKPLANE_PROXY_ENV_NAME,
			info.BACKPLANE_URL_ENV_NAME,
		)
		bpConfig, err = getEnvConfig()
		if err != nil {
			return bpConfig, err
		}

		return bpConfig, nil
	}

	// Load config file
	viper.SetConfigFile(filepath)

	err = viper.ReadInConfig()
	if err != nil {
		return bpConfig, err
	}

	err = viper.Unmarshal(&bpConfig)
	if err != nil {
		return bpConfig, err
	}

	ocmURL, err := getOCMUrl()
	if err != nil {
		return bpConfig, err
	}

	// Dynamically set Backplane URL based on OCM env
	switch ocmURL {
	case viper.GetString("ocm-prod"):
		bpConfig.URL = viper.GetString("bp-url-prod")
	case viper.GetString("ocm-stg"):
		bpConfig.URL = viper.GetString("bp-url-stg")
	case viper.GetString("ocm-int"):
		bpConfig.URL = viper.GetString("bp-url-int")
	default:
		bpConfig.URL = ""
	}

	if bpConfig.URL == "" {
		return bpConfig, fmt.Errorf("cannot specify Backplane URL, please make sure you are logged into OCM")
	}

	return bpConfig, nil
}
