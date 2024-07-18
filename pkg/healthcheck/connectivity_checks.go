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

type DefaultNetworkInterface struct{}
type DefaultHTTPClient struct {
	Client *http.Client
}

func (d DefaultNetworkInterface) Interfaces() ([]net.Interface, error) {
	return net.Interfaces()
}

func (d DefaultHTTPClient) Get(url string) (*http.Response, error) {
	return d.Client.Get(url)
}

var (
	netInterfaces            NetworkInterface = DefaultNetworkInterface{}
	httpClient               HTTPClient       = DefaultHTTPClient{Client: &http.Client{}}
	GetVPNCheckEndpointFunc                   = GetVPNCheckEndpoint
	GetProxyTestEndpointFunc                  = GetProxyTestEndpoint
)

// Dependency injection for testing purposes
var getConfigFunc = config.GetBackplaneConfiguration

func RunHealthCheck(checkVPN, checkProxy bool) func(cmd *cobra.Command, args []string) {
	return func(cmd *cobra.Command, args []string) {
		if checkVPN {
			fmt.Println("Checking VPN connectivity...")
			err := CheckVPNConnectivity(netInterfaces, httpClient)
			if err != nil {
				fmt.Println("VPN connectivity check failed:", err)
				os.Exit(1)
			} else {
				fmt.Println("VPN connectivity check passed!")
			}
			return
		}

		if checkProxy {
			err := CheckVPNConnectivity(netInterfaces, httpClient)
			if err != nil {
				fmt.Println("VPN connectivity check failed:", err)
				fmt.Println("Note: Proxy connectivity check requires VPN to be connected. Please ensure VPN is connected and try again.")
				os.Exit(1)
			}

			fmt.Println("Checking proxy connectivity...")
			_, err = CheckProxyConnectivity(httpClient)
			if err != nil {
				fmt.Println("Proxy connectivity check failed:", err)
				os.Exit(1)
			} else {
				fmt.Println("Proxy connectivity check passed!")
			}
			return
		}

		// If neither flag is set, check both VPN and Proxy connectivity, then check Backplane API connectivity.
		fmt.Println("Checking VPN connectivity...")
		err := CheckVPNConnectivity(netInterfaces, httpClient)
		if err != nil {
			fmt.Println("VPN connectivity check failed:", err)
			os.Exit(1)
		} else {
			fmt.Println("VPN connectivity check passed!")
		}

		fmt.Println("Checking proxy connectivity...")
		proxyURL, err := CheckProxyConnectivity(httpClient)
		if err != nil {
			fmt.Println("Proxy connectivity check failed:", err)
			os.Exit(1)
		} else {
			fmt.Println("Proxy connectivity check passed!")
		}

		fmt.Println("Checking backplane API connectivity...")
		err = CheckBackplaneAPIConnectivity(httpClient, proxyURL)
		if err != nil {
			fmt.Println("Backplane API connectivity check failed:", err)
			os.Exit(1)
		} else {
			fmt.Println("Backplane API connectivity check passed!")
		}
	}
}

func testEndPointConnectivity(testURL string, client HTTPClient) error {
	if client == nil {
		client = &DefaultHTTPClient{Client: &http.Client{}}
	}
	resp, err := client.Get(testURL)
	if err != nil {
		logger.Error("Failed to get response from test endpoint:", err)
		return err
	}
	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("Unexpected status code: %v", resp.StatusCode)
		logger.Error(errMsg)
		return fmt.Errorf(errMsg)
	}
	return nil
}

func CheckBackplaneAPIConnectivity(client HTTPClient, proxyURL string) error {
	bpConfig, err := getConfigFunc()
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
