package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/openshift/backplane-cli/pkg/info"
)

type BackplaneConfiguration struct {
	URL      string `json:"url"`
	ProxyURL string `json:"proxy-url"`
}

func GetBackplaneConfigFile() string {
	path, bpConfigFound := os.LookupEnv(info.BACKPLANE_CONFIG_PATH_ENV_NAME)
	if bpConfigFound {
		return path
	}

	return getConfiDefaultPath(info.BACKPLANE_CONFIG_DEFAULT_FILE_NAME)
}

// Get Backplane config default path
func getConfiDefaultPath(fileName string) string {
	UserHomeDir, err := os.UserHomeDir()
	if err != nil {
		return fileName
	}

	configFilePath := filepath.Join(UserHomeDir, ".config", fileName)

	return configFilePath
}

// Get Backplane ProxyUrl from config
func GetBackplaneConfiguration() (bpConfig BackplaneConfiguration, err error) {

	// Check proxy url from the config file
	filePath := GetBackplaneConfigFile()
	if _, err := os.Stat(filePath); err == nil {
		file, err := os.Open(filePath)

		if err != nil {
			return bpConfig, fmt.Errorf("failed to read file %s : %v", filePath, err)
		}

		defer file.Close()
		decoder := json.NewDecoder(file)
		bpConfig := BackplaneConfiguration{}
		err = decoder.Decode(&bpConfig)

		if err != nil {
			return bpConfig, fmt.Errorf("failed to decode file %s : %v", filePath, err)
		}

		return bpConfig, nil

	} else {
		// check proxy url from user perssitance HTTPS_PROXY env var
		proxyUrl, hasEnvProxyURL := os.LookupEnv(info.BACKPLANE_PROXY_ENV_NAME)

		// get backplane URL from BACKPLANE_URL env variables
		bpURL, hasURL := os.LookupEnv(info.BACKPLANE_URL_ENV_NAME)

		if hasURL {
			if bpURL == "" {
				return bpConfig, fmt.Errorf("%s env variable is empty", info.BACKPLANE_URL_ENV_NAME)
			}
			bpConfig.URL = bpURL
			if hasEnvProxyURL {
				bpConfig.ProxyURL = proxyUrl
			}

			return bpConfig, nil
		}
	}

	return bpConfig, nil
}
