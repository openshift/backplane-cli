package healthcheck

import (
	"fmt"
	"net/http"
	"net/url"

	logger "github.com/sirupsen/logrus"
)

// CheckProxyConnectivity checks the proxy connectivity
func CheckProxyConnectivity(client HTTPClient) (string, error) {
	logger.Debug("Starting CheckProxyConnectivity")
	bpConfig, err := GetConfigFunc()
	if err != nil {
		logger.Errorf("Failed to get backplane configuration: %v", err)
		return "", fmt.Errorf("failed to get backplane configuration: %v", err)
	}
	logger.Debugf("Backplane configuration: %+v", bpConfig)

	proxyURL := bpConfig.ProxyURL
	if proxyURL == nil || *proxyURL == "" {
		errMsg := "no proxy URL configured in backplane configuration"
		logger.Warn(errMsg)
		return "", fmt.Errorf(errMsg)
	}

	logger.Infof("Getting the working proxy URL ['%s'] from local backplane configuration.", *proxyURL)

	parsedProxyURL, err := url.Parse(*proxyURL)
	if err != nil {
		logger.Errorf("Invalid proxy URL: %v", err)
		return "", fmt.Errorf("invalid proxy URL: %v", err)
	}
	logger.Debugf("Parsed proxy URL: %s", parsedProxyURL)

	httpClientWithProxy := &DefaultHTTPClientImpl{
		Client: &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(parsedProxyURL),
			},
		},
	}
	logger.Debug("HTTP client with proxy configured")

	proxyTestEndpoint, err := GetProxyTestEndpointFunc()
	if err != nil {
		logger.Errorf("Failed to get proxy test endpoint: %v", err)
		return "", err
	}
	logger.Debugf("Proxy test endpoint: %s", proxyTestEndpoint)

	logger.Infof("Testing connectivity to the pre-defined test endpoint ['%s'] with the proxy.", proxyTestEndpoint)
	if err := testEndPointConnectivity(proxyTestEndpoint, httpClientWithProxy); err != nil {
		errMsg := fmt.Sprintf("Failed to access target endpoint ['%s'] with the proxy: %v", proxyTestEndpoint, err)
		logger.Errorf(errMsg)
		return "", fmt.Errorf(errMsg)
	}

	logger.Debugf("Successfully connected to proxy test endpoint: %s", proxyTestEndpoint)
	return *proxyURL, nil
}

func GetProxyTestEndpoint() (string, error) {
	bpConfig, err := GetConfigFunc()
	if err != nil {
		logger.Errorf("Failed to get backplane configuration: %v", err)
		return "", fmt.Errorf("failed to get backplane configuration: %v", err)
	}
	if bpConfig.ProxyCheckEndpoint == "" {
		errMsg := "proxy test endpoint not configured"
		logger.Warn(errMsg)
		return "", fmt.Errorf(errMsg)
	}
	return bpConfig.ProxyCheckEndpoint, nil
}
