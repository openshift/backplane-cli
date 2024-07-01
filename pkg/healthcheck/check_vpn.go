package healthcheck

import (
	"fmt"
	"strings"

	logger "github.com/sirupsen/logrus"
)

// CheckVPNConnectivity checks the VPN connectivity
func CheckVPNConnectivity(netInterfaces NetworkInterface, client HTTPClient) error {
	vpnInterfaces := []string{"tun", "tap", "ppp", "wg", "utun"}

	interfaces, err := netInterfaces.Interfaces()
	if err != nil {
		logger.Errorf("Failed to get network interfaces: %v", err)
		return fmt.Errorf("failed to get network interfaces: %v", err)
	}

	vpnConnected := false
	for _, iface := range interfaces {
		for _, vpnPrefix := range vpnInterfaces {
			if strings.HasPrefix(iface.Name, vpnPrefix) {
				vpnConnected = true
				break
			}
		}
		if vpnConnected {
			break
		}
	}

	if !vpnConnected {
		errMsg := fmt.Sprintf("No VPN interfaces found: %v", vpnInterfaces)
		logger.Warn(errMsg)
		return fmt.Errorf(errMsg)
	}

	vpnCheckEndpoint, err := GetVPNCheckEndpointFunc()
	if err != nil {
		logger.Errorf("Failed to get VPN check endpoint: %v", err)
		return err
	}
	if err := testEndPointConnectivity(vpnCheckEndpoint, client); err != nil {
		errMsg := fmt.Sprintf("Failed to access internal URL %s: %v", vpnCheckEndpoint, err)
		logger.Errorf(errMsg)
		return fmt.Errorf(errMsg)
	}

	return nil
}

func GetVPNCheckEndpoint() (string, error) {
	bpConfig, err := getConfigFunc()
	if err != nil {
		logger.Errorf("Failed to get backplane configuration: %v", err)
		return "", fmt.Errorf("failed to get backplane configuration: %v", err)
	}
	if bpConfig.VPNCheckEndpoint == "" {
		errMsg := "VPN check endpoint not configured"
		logger.Warn(errMsg)
		return "", fmt.Errorf(errMsg)
	}
	return bpConfig.VPNCheckEndpoint, nil
}
