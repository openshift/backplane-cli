package healthcheck

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/healthcheck/mocks"
)

func TestCheckVPNConnectivity(t *testing.T) {
	tests := []struct {
		name        string
		interfaces  []net.Interface
		vpnEndpoint string
		expectErr   bool
	}{
		{
			name: "VPN connected - Linux",
			interfaces: []net.Interface{
				{Name: "tun0"},
			},
			vpnEndpoint: "http://vpn-endpoint",
			expectErr:   false,
		},
		{
			name: "VPN connected - macOS",
			interfaces: []net.Interface{
				{Name: "utun0"},
			},
			vpnEndpoint: "http://vpn-endpoint",
			expectErr:   false,
		},
		{
			name: "VPN not connected",
			interfaces: []net.Interface{
				{Name: "eth0"},
			},
			vpnEndpoint: "http://vpn-endpoint",
			expectErr:   true,
		},
		{
			name:       "No VPN interfaces",
			interfaces: []net.Interface{},
			expectErr:  true,
		},
	}

	originalGetVPNCheckEndpointFunc := GetVPNCheckEndpointFunc
	defer func() { GetVPNCheckEndpointFunc = originalGetVPNCheckEndpointFunc }()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			GetVPNCheckEndpointFunc = func() (string, error) {
				if tt.vpnEndpoint == "" {
					return "", errors.New("VPN check endpoint not configured")
				}
				return tt.vpnEndpoint, nil
			}

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			netInterfaces := mocks.NewMockNetworkInterface(mockCtrl)
			netInterfaces.EXPECT().Interfaces().Return(tt.interfaces, nil).AnyTimes()

			mockHTTPClient := mocks.NewMockHTTPClient(mockCtrl)
			mockHTTPClient.EXPECT().Get(gomock.Any()).Return(&http.Response{StatusCode: http.StatusOK}, nil).AnyTimes()

			if tt.vpnEndpoint != "" {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
				defer server.Close()
				tt.vpnEndpoint = server.URL
			}

			err := CheckVPNConnectivity(netInterfaces, mockHTTPClient)
			if (err != nil) != tt.expectErr {
				t.Errorf("CheckVPNConnectivity() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestGetVPNCheckEndpoint(t *testing.T) {
	originalGetConfigFunc := getConfigFunc
	defer func() { getConfigFunc = originalGetConfigFunc }()

	tests := []struct {
		name      string
		config    config.BackplaneConfiguration
		expectErr bool
	}{
		{
			name: "Configured VPN endpoint",
			config: config.BackplaneConfiguration{
				VPNCheckEndpoint: "http://vpn-endpoint",
			},
			expectErr: false,
		},
		{
			name: "No VPN endpoint configured",
			config: config.BackplaneConfiguration{
				VPNCheckEndpoint: "",
			},
			expectErr: true,
		},
		{
			name:      "Failed to get backplane configuration",
			config:    config.BackplaneConfiguration{},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getConfigFunc = func() (config.BackplaneConfiguration, error) {
				if tt.name == "Failed to get backplane configuration" {
					return config.BackplaneConfiguration{}, errors.New("failed to get backplane configuration")
				}
				return tt.config, nil
			}

			_, err := GetVPNCheckEndpoint()
			if (err != nil) != tt.expectErr {
				t.Errorf("GetVPNCheckEndpoint() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}
