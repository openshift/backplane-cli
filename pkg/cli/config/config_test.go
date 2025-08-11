package config

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
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

	t.Run("should pass for valid http proxy URL", func(t *testing.T) {
		proxyURL := "http://www.example.com"
		config := BackplaneConfiguration{URL: "https://api.example.com", ProxyURL: &proxyURL}
		_, err := config.testHTTPRequestToBackplaneAPI()
		
		// Should not get scheme validation error (DNS lookup error is expected)
		if err != nil && strings.Contains(err.Error(), "proxy URL scheme must be http or https") {
			t.Errorf("unexpected scheme validation error: %v", err)
		}
	})

	t.Run("should pass for valid https proxy URL", func(t *testing.T) {
		proxyURL := "https://www.example.com"
		config := BackplaneConfiguration{URL: "https://api.example.com", ProxyURL: &proxyURL}
		_, err := config.testHTTPRequestToBackplaneAPI()
		
		// Should not get scheme validation error (DNS lookup error is expected)
		if err != nil && strings.Contains(err.Error(), "proxy URL scheme must be http or https") {
			t.Errorf("unexpected scheme validation error: %v", err)
		}
	})

	t.Run("should fail for proxy URL without scheme", func(t *testing.T) {
		proxyURL := "www.example.com"
		config := BackplaneConfiguration{URL: "https://api.example.com", ProxyURL: &proxyURL}
		_, err := config.testHTTPRequestToBackplaneAPI()
		
		if err == nil {
			t.Errorf("expected error but got none")
		} else if !strings.Contains(err.Error(), "proxy URL scheme must be http or https, got:") {
			t.Errorf("expected scheme validation error, got: %s", err.Error())
		}
	})

	t.Run("should fail for ftp proxy URL", func(t *testing.T) {
		proxyURL := "ftp://www.example.com"
		config := BackplaneConfiguration{URL: "https://api.example.com", ProxyURL: &proxyURL}
		_, err := config.testHTTPRequestToBackplaneAPI()
		
		if err == nil {
			t.Errorf("expected error but got none")
		} else if !strings.Contains(err.Error(), "proxy URL scheme must be http or https, got: ftp") {
			t.Errorf("expected scheme validation error for ftp, got: %s", err.Error())
		}
	})

	t.Run("should fail on DNS lookup error", func(t *testing.T) {
		// Mock DNS lookup to return an error
		originalLookupHost := lookupHost
		lookupHost = func(hostname string) ([]string, error) {
			return nil, fmt.Errorf("DNS resolution failed")
		}
		defer func() { lookupHost = originalLookupHost }()

		proxyURL := "https://proxy.example.com"
		config := BackplaneConfiguration{URL: "https://api.example.com", ProxyURL: &proxyURL}
		_, err := config.testHTTPRequestToBackplaneAPI()
		
		if err == nil {
			t.Errorf("expected DNS lookup error but got none")
		} else if !strings.Contains(err.Error(), "DNS lookup failed for proxy hostname") {
			t.Errorf("expected DNS lookup error, got: %s", err.Error())
		}
	})

	t.Run("should pass on successful DNS lookup", func(t *testing.T) {
		// Mock DNS lookup to succeed
		originalLookupHost := lookupHost
		lookupHost = func(hostname string) ([]string, error) {
			return []string{"192.168.1.1"}, nil
		}
		defer func() { lookupHost = originalLookupHost }()

		proxyURL := "https://proxy.example.com"
		config := BackplaneConfiguration{URL: "https://api.example.com", ProxyURL: &proxyURL}
		_, err := config.testHTTPRequestToBackplaneAPI()
		
		// Should not get DNS lookup error (HTTP request error is expected)
		if err != nil && strings.Contains(err.Error(), "DNS lookup failed for proxy hostname") {
			t.Errorf("unexpected DNS lookup error: %v", err)
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
