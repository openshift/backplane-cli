package config

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/spf13/viper"

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

	t.Run("display cluster info is true", func(t *testing.T) {
		viper.Set("display-cluster-info", true)
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

		if !config.DisplayClusterInfo {
			t.Errorf("expected DisplayClusterInfo to be true, got %v", config.DisplayClusterInfo)
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
		name     string
		proxies  []string
		testFunc func(ctx context.Context, testURL string, proxyURL url.URL) error
		want     string
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
			name:    "single-valid-proxy",
			proxies: []string{"https://proxy.invalid"},
			testFunc: func(ctx context.Context, testURL string, proxyURL url.URL) error {
				return nil
			},
			want: "https://proxy.invalid",
		},
		{
			name:    "one-proxy-fails",
			proxies: []string{"http://this.proxy.succeeds", "http://this.proxy.fails"},
			testFunc: func(ctx context.Context, testURL string, proxyURL url.URL) error {
				if proxyURL.Host == "this.proxy.succeeds" {
					return nil
				}

				return fmt.Errorf("Testing Error")
			},
			want: "http://this.proxy.succeeds",
		},
		{
			name:    "multiple-mixed-proxies",
			proxies: []string{"-", "gellso", "https://proxy.invalid"},
			testFunc: func(ctx context.Context, testURL string, proxyURL url.URL) error {
				return nil
			},
			want: "https://proxy.invalid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("dummy data"))
			}))

			testProxy = tt.testFunc

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
				_ = os.Setenv(info.BackplaneProxyEnvName, tt.envProxy)
			} else {
				_ = os.Unsetenv(info.BackplaneProxyEnvName)
			}

			// Validate config
			err := validateConfig()
			if (err != nil) != tt.expectError {
				t.Errorf("expected error: %v, got: %v", tt.expectError, err)
			}
		})
	}
}
