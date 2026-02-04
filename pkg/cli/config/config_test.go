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

	t.Run("it reads JIRA token from JIRA_API_TOKEN environment variable when config file is empty", func(t *testing.T) {
		viper.Reset()
		svr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("dummy data"))
		}))

		expectedToken := "test-jira-token-12345"
		userDefinedProxy := "example-proxy"
		t.Setenv("BACKPLANE_URL", svr.URL)
		t.Setenv("HTTPS_PROXY", userDefinedProxy)
		t.Setenv("JIRA_API_TOKEN", expectedToken)

		config, err := GetBackplaneConfiguration()
		if err != nil {
			t.Error(err)
		}

		if config.JiraToken != expectedToken {
			t.Errorf("expected JiraToken to be %s, got %s", expectedToken, config.JiraToken)
		}
	})

	t.Run("JIRA_API_TOKEN environment variable takes precedence over config file JIRA token", func(t *testing.T) {
		viper.Reset()
		svr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("dummy data"))
		}))

		configToken := "config-file-token"    //nolint:gosec
		envToken := "env-var-token-wins" //nolint:gosec

		t.Setenv("BACKPLANE_URL", svr.URL)
		t.Setenv("JIRA_API_TOKEN", envToken)

		// Simulate config file value
		viper.Set(JiraTokenViperKey, configToken)

		config, err := GetBackplaneConfiguration()
		if err != nil {
			t.Error(err)
		}

		if config.JiraToken != envToken {
			t.Errorf("expected environment variable token to take precedence: expected %s, got %s", envToken, config.JiraToken)
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

// TestNoAutomaticConfigFetch verifies that GetBackplaneConfiguration
// does NOT automatically fetch config from the API when values are missing
func TestNoAutomaticConfigFetch(t *testing.T) {
	t.Run("uses default values when config file is missing", func(t *testing.T) {
		viper.Reset()

		svr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// This should NOT be called - if it is, the test will fail
			t.Error("HTTP request was made - automatic config fetch detected!")
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer svr.Close()

		userDefinedProxy := "example-proxy"
		t.Setenv("BACKPLANE_URL", svr.URL)
		t.Setenv("HTTPS_PROXY", userDefinedProxy)

		config, err := GetBackplaneConfiguration()
		if err != nil {
			t.Error(err)
		}

		// Should use default values, not fetch from API
		if config.JiraBaseURL != JiraBaseURLDefaultValue {
			t.Errorf("expected default JiraBaseURL %s, got %s", JiraBaseURLDefaultValue, config.JiraBaseURL)
		}
		if config.ProdEnvName != "production" {
			t.Errorf("expected default ProdEnvName 'production', got %s", config.ProdEnvName)
		}
	})

	t.Run("uses config file values without fetching from API", func(t *testing.T) {
		viper.Reset()

		// Create a temp config file with specific values
		tmpDir := t.TempDir()
		configPath := tmpDir + "/config.json"
		configContent := `{
			"jira-base-url": "https://custom-jira.example.com",
			"assume-initial-arn": "arn:aws:iam::999999999:role/Custom-Role",
			"prod-env-name": "custom-prod"
		}`
		err := os.WriteFile(configPath, []byte(configContent), 0600)
		if err != nil {
			t.Fatal(err)
		}

		t.Setenv("BACKPLANE_CONFIG", configPath)

		svr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// This should NOT be called
			t.Error("HTTP request was made - automatic config fetch detected!")
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer svr.Close()

		userDefinedProxy := "example-proxy"
		t.Setenv("BACKPLANE_URL", svr.URL)
		t.Setenv("HTTPS_PROXY", userDefinedProxy)

		config, err := GetBackplaneConfiguration()
		if err != nil {
			t.Error(err)
		}

		// Should use values from config file
		if config.JiraBaseURL != "https://custom-jira.example.com" {
			t.Errorf("expected JiraBaseURL from config file, got %s", config.JiraBaseURL)
		}
		if config.AssumeInitialArn != "arn:aws:iam::999999999:role/Custom-Role" {
			t.Errorf("expected AssumeInitialArn from config file, got %s", config.AssumeInitialArn)
		}
		if config.ProdEnvName != "custom-prod" {
			t.Errorf("expected ProdEnvName from config file, got %s", config.ProdEnvName)
		}
	})

	t.Run("partial config file uses defaults for missing values without API fetch", func(t *testing.T) {
		viper.Reset()

		// Create a config file with only some values
		tmpDir := t.TempDir()
		configPath := tmpDir + "/config.json"
		configContent := `{
			"jira-base-url": "https://partial-jira.example.com"
		}`
		err := os.WriteFile(configPath, []byte(configContent), 0600)
		if err != nil {
			t.Fatal(err)
		}

		t.Setenv("BACKPLANE_CONFIG", configPath)

		svr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// This should NOT be called
			t.Error("HTTP request was made - automatic config fetch detected!")
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer svr.Close()

		userDefinedProxy := "example-proxy"
		t.Setenv("BACKPLANE_URL", svr.URL)
		t.Setenv("HTTPS_PROXY", userDefinedProxy)

		config, err := GetBackplaneConfiguration()
		if err != nil {
			t.Error(err)
		}

		// Should use value from config file
		if config.JiraBaseURL != "https://partial-jira.example.com" {
			t.Errorf("expected JiraBaseURL from config file, got %s", config.JiraBaseURL)
		}

		// Should use defaults for missing values, NOT fetch from API
		if config.ProdEnvName != "production" {
			t.Errorf("expected default ProdEnvName 'production', got %s", config.ProdEnvName)
		}
	})
}
