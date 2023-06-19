package config

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

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

// CheckAPIConnection validate API connection via configured proxy and VPN
func (config BackplaneConfiguration) CheckAPIConnection() error {

	// Check backplane Proxy URL
	if config.ProxyURL == "" {
		path, err := GetConfigFilePath()
		if err != nil {
			return err
		}
		return errors.New("empty proxy url - check your backplane-cli configuration in " + path)
	}

	// make test api connection
	connectionOk, err := config.testHttpRequestToBackplaneAPI()

	if !connectionOk {
		return err
	}

	return nil
}

// testHttpRequestToBackplaneAPI returns status of the the API connection
func (config BackplaneConfiguration) testHttpRequestToBackplaneAPI() (bool, error) {
	client := http.Client{
		Timeout: 5 * time.Second,
	}

	if config.ProxyURL != "" {
		proxyUrl, err := url.Parse(config.ProxyURL)
		if err != nil {
			return false, err
		}
		http.DefaultTransport = &http.Transport{Proxy: http.ProxyURL(proxyUrl)}
	}

	req, err := http.NewRequest("HEAD", config.URL, nil)
	if err != nil {
		return false, err
	}
	_, err = client.Do(req)
	if err != nil {
		return false, err
	}

	return true, nil
}
