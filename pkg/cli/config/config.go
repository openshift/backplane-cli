package config

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	logger "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/openshift/backplane-cli/pkg/info"
)

type BackplaneConfiguration struct {
	URL              string
	ProxyURL         *string // Optional
	SessionDirectory string
	AssumeInitialArn string
}

// GetConfigFilePath returns the Backplane CLI configuration filepath
func GetConfigFilePath() (string, error) {
	// Check if user has explicitly defined backplane config path
	path, found := os.LookupEnv(info.BackplaneConfigPathEnvName)
	if found {
		return path, nil
	}

	UserHomeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	configFilePath := filepath.Join(UserHomeDir, info.BackplaneConfigDefaultFilePath, info.BackplaneConfigDefaultFileName)

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
	err = viper.BindEnv("url", info.BackplaneURLEnvName)
	if err != nil {
		return bpConfig, err
	}

	// Check if user has explicitly defined proxy; it has higher precedence over the config file
	err = viper.BindEnv("proxy-url", info.BackplaneProxyEnvName)
	if err != nil {
		return bpConfig, err
	}

	bpConfig.URL = viper.GetString("url")
	bpConfig.SessionDirectory = viper.GetString("session-dir")
	bpConfig.AssumeInitialArn = viper.GetString("assume-initial-arn")

	// proxyURL is optional
	proxyURL := viper.GetString("proxy-url")
	if proxyURL != "" {
		bpConfig.ProxyURL = &proxyURL
	} else {
		logger.Warn("No proxy configuration available. This may result in failing commands as backplane-api is only available from select networks.")
	}

	return bpConfig, nil
}

// CheckAPIConnection validate API connection via configured proxy and VPN
func (config BackplaneConfiguration) CheckAPIConnection() error {

	// make test api connection
	connectionOk, err := config.testHTTPRequestToBackplaneAPI()

	if !connectionOk {
		return err
	}

	return nil
}

// testHTTPRequestToBackplaneAPI returns status of the API connection
func (config BackplaneConfiguration) testHTTPRequestToBackplaneAPI() (bool, error) {
	client := http.Client{
		Timeout: 5 * time.Second,
	}

	if config.ProxyURL != nil {
		proxyURL, err := url.Parse(*config.ProxyURL)
		if err != nil {
			return false, err
		}
		http.DefaultTransport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
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
