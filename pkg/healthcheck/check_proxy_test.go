package healthcheck

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/healthcheck/mocks"
)

func TestCheckProxyConnectivity(t *testing.T) {
	tests := []struct {
		name          string
		proxyURL      string
		proxyEndpoint string
		expectErr     bool
	}{
		{
			name:      "Proxy not configured",
			proxyURL:  "",
			expectErr: true,
		},
	}

	originalGetProxyTestEndpointFunc := GetProxyTestEndpointFunc
	defer func() { GetProxyTestEndpointFunc = originalGetProxyTestEndpointFunc }()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			GetProxyTestEndpointFunc = func() (string, error) {
				if tt.proxyEndpoint == "" {
					return "", errors.New("proxy test endpoint not configured")
				}
				return tt.proxyEndpoint, nil
			}

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			client := mocks.NewMockHTTPClient(mockCtrl)
			client.EXPECT().Get(gomock.Any()).Return(&http.Response{StatusCode: http.StatusOK}, nil).AnyTimes()

			url, err := CheckProxyConnectivity(client)
			if (err != nil) != tt.expectErr {
				t.Errorf("CheckProxyConnectivity() error = %v, expectErr %v", err, tt.expectErr)
			}
			if err == nil && url != tt.proxyURL {
				t.Errorf("Expected proxy URL = %v, got %v", tt.proxyURL, url)
			}
		})
	}
}

func TestCheckBackplaneAPIConnectivity(t *testing.T) {
	tests := []struct {
		name      string
		proxyURL  string
		apiURL    string
		expectErr bool
	}{
		{
			name:      "API not accessible through proxy",
			proxyURL:  "http://proxy:8080",
			apiURL:    "http://bad-api-endpoint",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.String() == tt.apiURL {
					w.WriteHeader(http.StatusOK)
				} else {
					w.WriteHeader(http.StatusInternalServerError)
				}
			}))
			defer server.Close()
			tt.apiURL = server.URL

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			client := mocks.NewMockHTTPClient(mockCtrl)
			client.EXPECT().Get(gomock.Any()).Return(&http.Response{StatusCode: http.StatusOK}, nil).AnyTimes()

			err := CheckBackplaneAPIConnectivity(client, tt.proxyURL)
			if (err != nil) != tt.expectErr {
				t.Errorf("CheckBackplaneAPIConnectivity() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestGetProxyTestEndpoint(t *testing.T) {
	originalGetConfigFunc := getConfigFunc
	defer func() { getConfigFunc = originalGetConfigFunc }()

	tests := []struct {
		name      string
		config    config.BackplaneConfiguration
		expectErr bool
	}{
		{
			name: "Configured proxy endpoint",
			config: config.BackplaneConfiguration{
				ProxyCheckEndpoint: "http://proxy-endpoint",
			},
			expectErr: false,
		},
		{
			name: "No proxy endpoint configured",
			config: config.BackplaneConfiguration{
				ProxyCheckEndpoint: "",
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

			_, err := GetProxyTestEndpoint()
			if (err != nil) != tt.expectErr {
				t.Errorf("GetProxyTestEndpoint() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}
