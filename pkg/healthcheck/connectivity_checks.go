package healthcheck

import (
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/openshift/backplane-cli/pkg/cli/config"
	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// Interfaces for dependencies
type NetworkInterface interface {
	Interfaces() ([]net.Interface, error)
}

type HTTPClient interface {
	Get(url string) (*http.Response, error)
}

// Default implementations
type DefaultNetworkInterfaceImpl struct{}

type DefaultHTTPClientImpl struct {
	Client *http.Client
}

func (d DefaultNetworkInterfaceImpl) Interfaces() ([]net.Interface, error) {
	return net.Interfaces()
}

func (d DefaultHTTPClientImpl) Get(url string) (*http.Response, error) {
	return d.Client.Get(url)
}

var (
	NetInterfaces            NetworkInterface = DefaultNetworkInterfaceImpl{}
	HTTPClients              HTTPClient       = DefaultHTTPClientImpl{Client: &http.Client{}}
	GetVPNCheckEndpointFunc                   = GetVPNCheckEndpoint
	GetProxyTestEndpointFunc                  = GetProxyTestEndpoint
	GetConfigFunc                             = config.GetBackplaneConfiguration
)

func RunHealthCheck(checkVPN, checkProxy bool) func(cmd *cobra.Command, args []string) {
	return func(cmd *cobra.Command, args []string) {
		if NetInterfaces == nil || HTTPClients == nil {
			logger.Error("Network interfaces or HTTP client is not configured")
			os.Exit(1)
		}

		if checkVPN {
			fmt.Println("Checking VPN connectivity...")
			err := CheckVPNConnectivity(NetInterfaces, HTTPClients)
			if err != nil {
				fmt.Println("VPN connectivity check failed:", err)
				os.Exit(1)
			} else {
				fmt.Println("VPN connectivity check passed!")
			}
			return
		}

		if checkProxy {
			err := CheckVPNConnectivity(NetInterfaces, HTTPClients)
			if err != nil {
				fmt.Println("VPN connectivity check failed:", err)
				fmt.Println("Note: Proxy connectivity check requires VPN to be connected. Please ensure VPN is connected and try again.")
				os.Exit(1)
			}

			fmt.Println("Checking proxy connectivity...")
			_, err = CheckProxyConnectivity(HTTPClients)
			if err != nil {
				fmt.Println("Proxy connectivity check failed:", err)
				os.Exit(1)
			} else {
				fmt.Println("Proxy connectivity check passed!")
			}
			return
		}

		// If neither flag is set, check both VPN and Proxy connectivity, then check Backplane API connectivity.
		checkAllConnections()
	}
}

func checkAllConnections() {
	fmt.Println("Checking VPN connectivity...")
	err := CheckVPNConnectivity(NetInterfaces, HTTPClients)
	if err != nil {
		fmt.Println("VPN connectivity check failed:", err)
		os.Exit(1)
	} else {
		fmt.Println("VPN connectivity check passed!")
	}

	fmt.Println("Checking proxy connectivity...")
	proxyURL, err := CheckProxyConnectivity(HTTPClients)
	if err != nil {
		fmt.Println("Proxy connectivity check failed:", err)
		os.Exit(1)
	} else {
		fmt.Println("Proxy connectivity check passed!")
	}

	fmt.Println("Checking backplane API connectivity...")
	err = CheckBackplaneAPIConnectivity(HTTPClients, proxyURL)
	if err != nil {
		fmt.Println("Backplane API connectivity check failed:", err)
		os.Exit(1)
	} else {
		fmt.Println("Backplane API connectivity check passed!")
	}
}

func testEndPointConnectivity(testURL string, client HTTPClient) error {
	if client == nil {
		client = &DefaultHTTPClientImpl{Client: &http.Client{}}
	}
	logger.Debugf("Making GET request to %s", testURL)
	resp, err := client.Get(testURL)
	if err != nil {
		logger.Error("Failed to get response from test endpoint:", err)
		return err
	}
	logger.Debugf("Received response status code: %d", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("Unexpected status code: %v", resp.StatusCode)
		logger.Error(errMsg)
		return fmt.Errorf("%s", errMsg)
	}
	return nil
}

func CheckBackplaneAPIConnectivity(client HTTPClient, proxyURL string) error {
	logger.Debug("Starting CheckBackplaneAPIConnectivity")
	bpConfig, err := GetConfigFunc()
	if err != nil {
		logger.Error("Failed to get backplane configuration:", err)
		return fmt.Errorf("failed to get backplane configuration: %v", err)
	}

	if proxyURL != "" {
		bpConfig.ProxyURL = &proxyURL
	}

	err = bpConfig.CheckAPIConnection()
	if err != nil {
		logger.Error("Failed to access backplane API:", err)
		return fmt.Errorf("failed to access backplane API: %v", err)
	}

	fmt.Println("Successfully connected to the backplane API!")
	return nil
}
