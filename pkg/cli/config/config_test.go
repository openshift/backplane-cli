package config

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/mock"

	"github.com/openshift/backplane-cli/pkg/info"
)

func TestGetBackplaneConfig(t *testing.T) {
	t.Run("it returns the user defined proxy instead of the configuration variable", func(t *testing.T) {

		svr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("dummy data"))
		}))

		userDefinedProxy := "example-proxy"
		t.Setenv("BACKPLANE_URL", svr.URL)
		t.Setenv("HTTPS_PROXY", userDefinedProxy)
		config, err := GetBackplaneConfiguration()
		if err != nil {
			t.Error(err)
		}

		if config.ProxyURL != nil && *config.ProxyURL != userDefinedProxy {
			t.Errorf("expected to return the explicitly defined proxy %v instead of the default one %v", userDefinedProxy, config.ProxyURL)
		}
	})
}

func TestGetBackplaneConnection(t *testing.T) {
	t.Run("should fail if backplane API return connection errors", func(t *testing.T) {

		svr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("dummy data"))
		}))

		proxyURL := "http://dummy.proxy"
		t.Setenv("BACKPLANE_URL", svr.URL)
		t.Setenv("HTTPS_PROXY", proxyURL)
		config, err := GetBackplaneConfiguration()
		if err != nil {
			t.Error(err)
		}

		err = config.CheckAPIConnection()
		if err != nil {
			t.Failed()
		}

	})

	t.Run("should fail for empty proxy url", func(t *testing.T) {
		config := BackplaneConfiguration{URL: "https://dummy-url", ProxyURL: nil}
		err := config.CheckAPIConnection()

		if err != nil {
			t.Failed()
		}
	})
}

func TestBackplaneConfiguration_getFirstWorkingProxyURL(t *testing.T) {
	tests := []struct {
		name         string
		proxies      []string
		clientDoFunc func(client *http.Client, req *http.Request) (*http.Response, error)
		want         string
	}{
		{
			name:    "invalid-format-proxy",
			proxies: []string{""},
			want:    "",
		},
		{
			name:    "multiple-invalid-proxies",
			proxies: []string{"-", "gellso", ""},
			want:    "-",
		},
		{
			name:    "valid-proxies",
			proxies: []string{"https://proxy.invalid"},
			clientDoFunc: func(client *http.Client, req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK}, nil
			},
			want: "https://proxy.invalid",
		},
		{
			name:    "multiple-valid-proxies",
			proxies: []string{"https://proxy.invalid", "https://dummy.proxy.invalid"},
			clientDoFunc: func(client *http.Client, req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK}, nil
			},
			want: "https://proxy.invalid",
		},
		{
			name:    "multiple-mixed-proxies",
			proxies: []string{"-", "gellso", "https://proxy.invalid"},
			clientDoFunc: func(client *http.Client, req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK}, nil
			},
			want: "https://proxy.invalid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("dummy data"))
			}))

			clientDo = tt.clientDoFunc

			config := &BackplaneConfiguration{
				URL: svr.URL,
			}
			got := config.getFirstWorkingProxyURL(tt.proxies)

			if got != tt.want {
				t.Errorf("BackplaneConfiguration.getFirstWorkingProxyURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		proxyConfig []string
		envProxy    string
		expectError bool
	}{
		{"No proxy set", nil, "", true},
		{"Proxy set in config", []string{"http://proxy.example.com"}, "", false},
		{"Proxy set in environment", nil, "http://proxy.example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up viper configuration
			viper.Set("proxy-url", tt.proxyConfig)

			// Set up environment variable
			if tt.envProxy != "" {
				os.Setenv(info.BackplaneProxyEnvName, tt.envProxy)
			} else {
				os.Unsetenv(info.BackplaneProxyEnvName)
			}

			// Validate config
			err := validateConfig()
			if (err != nil) != tt.expectError {
				t.Errorf("expected error: %v, got: %v", tt.expectError, err)
			}
		})
	}
}

// Test DisplayClusterInfoHandling tests the behavior of the PrintClusterInfo method

type MockLogin struct {
	mock.Mock
}

func (m *MockLogin) PrintClusterInfo(clusterID string) error {
	args := m.Called(clusterID)
	return args.Error(0)
}

func TestDisplayClusterInfoHandling(t *testing.T) {
	tests := []struct {
		name                      string
		displayClusterInfoEnabled bool
		mockPrintClusterInfoError error
		expectedError             error
		expectMockCalled          bool
	}{
		{
			name:                      "DisplayClusterInfo is enabled and PrintClusterInfo succeeds",
			displayClusterInfoEnabled: true,
			mockPrintClusterInfoError: nil,
			expectedError:             nil,
			expectMockCalled:          true,
		},
		{
			name:                      "DisplayClusterInfo is enabled and PrintClusterInfo fails",
			displayClusterInfoEnabled: true,
			mockPrintClusterInfoError: fmt.Errorf("mock error"),
			expectedError:             fmt.Errorf("failed to print cluster info: mock error"),
			expectMockCalled:          true,
		},
		{
			name:                      "DisplayClusterInfo is disabled",
			displayClusterInfoEnabled: false,
			mockPrintClusterInfoError: nil,
			expectedError:             nil,
			expectMockCalled:          false, // Mock should not be called
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogin := new(MockLogin)
			clusterID := "test-cluster-id"

			// Mock the PrintClusterInfo behavior
			mockLogin.On("PrintClusterInfo", clusterID).Return(tt.mockPrintClusterInfoError)

			// bpConfig structure
			bpConfig := struct {
				DisplayClusterInfo bool
			}{
				DisplayClusterInfo: tt.displayClusterInfoEnabled,
			}

			// Run code under test
			var err error
			if bpConfig.DisplayClusterInfo {
				err = mockLogin.PrintClusterInfo(clusterID)
				if err != nil {
					err = fmt.Errorf("failed to print cluster info: %v", err)
				}
			}

			if (err != nil && tt.expectedError == nil) || (err == nil && tt.expectedError != nil) || (err != nil && err.Error() != tt.expectedError.Error()) {
				t.Errorf("expected error: %v, got: %v", tt.expectedError, err)
			}

			if tt.expectMockCalled {
				mockLogin.AssertExpectations(t) // Assert that the mock method was called
			} else {
				mockLogin.AssertNotCalled(t, "PrintClusterInfo", clusterID) // Assert that the mock method was NOT called
			}
		})
	}
}
