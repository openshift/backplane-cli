package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/openshift/backplane-cli/pkg/info"
)

type BackplaneConfiguration struct {
	URL       string
	Proxy_URL string
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
	configDir, err := os.UserConfigDir()

	if err != nil {
		return fileName
	}

	return configDir + "/" + fileName
}

// Get Backplane ProxyUrl from config
func GetBackplaneProxyUrl() (proxyUrl string, err error) {

	// Check proxy url from the config file
	filePath := GetBackplaneConfigFile()
	if _, err := os.Stat(filePath); err == nil {
		file, err := os.Open(filePath)

		if err != nil {
			return proxyUrl, fmt.Errorf("failed to read file %s : %v", filePath, err)
		}

		defer file.Close()
		decoder := json.NewDecoder(file)
		bpConfig := BackplaneConfiguration{}
		err = decoder.Decode(&bpConfig)

		if err != nil {
			return proxyUrl, fmt.Errorf("failed to decode file %s : %v", filePath, err)
		}
		proxyUrl = bpConfig.Proxy_URL

	} else {
		// check proxy url from user perssitance HTTPS_PROXY env var
		proxyUrl, hasEnvProxyURL := os.LookupEnv(info.BACKPLANE_PROXY_ENV_NAME)
		if hasEnvProxyURL {
			return proxyUrl, nil
		}
	}

	return proxyUrl, nil
}
