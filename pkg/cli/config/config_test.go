package config

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetBackplaneConfig(t *testing.T) {
	t.Run("it returns the user defined proxy instead of the configuration variable", func(t *testing.T) {
		userDefinedProxy := "example-proxy"
		t.Setenv("HTTPS_PROXY", userDefinedProxy)
		config, err := GetBackplaneConfiguration()
		if err != nil {
			t.Error(err)
		}

		if config.ProxyURL != nil && *config.ProxyURL != userDefinedProxy {
			t.Errorf("expected to return the explicitly defined proxy %v instead of the default one %v", userDefinedProxy, config.ProxyURL)
		}
	})

	t.Run("it returns the user defined backplane URL instead of the configuration variable", func(t *testing.T) {
		userDefinedURL := "example-url"
		t.Setenv("BACKPLANE_URL", userDefinedURL)
		config, err := GetBackplaneConfiguration()
		if err != nil {
			t.Error(err)
		}

		if config.URL != userDefinedURL {
			t.Errorf("expected to return the explicitly defined url %v instead of the default one %v", userDefinedURL, config.URL)
		}
	})
}

func TestGetBackplaneConfiguration(t *testing.T) {

	for name, tc := range map[string]struct {
		envNeedToSet         bool
		backplaneURLEnvVar   string
		proxyURL             string
		expectedBackplaneURL string
		expectedError        bool
	}{
		"backplane url set via env vars": {
			envNeedToSet:         true,
			backplaneURLEnvVar:   "https://api-backplane.apps.openshiftapps.com",
			proxyURL:             "http://squid.myproxy.com",
			expectedBackplaneURL: "https://api-backplane.apps.openshiftapps.com",
			expectedError:        false,
		},
	} {
		tc := tc

		t.Run(name, func(t *testing.T) {
			if tc.envNeedToSet {
				t.Setenv("BACKPLANE_URL", tc.backplaneURLEnvVar)
				t.Setenv("HTTPS_PROXY", tc.proxyURL)
			}

			bpConfig, err := GetBackplaneConfiguration()

			if tc.expectedError && err == nil {
				t.Errorf("expected err to be %v", err)
			}
			if bpConfig.URL != tc.expectedBackplaneURL {
				t.Errorf("expected res to be %s got %s", tc.expectedBackplaneURL, bpConfig.URL)
			}
		})
	}
}

func TestGetBackplaneConnection(t *testing.T) {
	t.Run("should fail if backplane API return connection errors", func(t *testing.T) {

		svr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("dummy data"))
		}))

		proxyURL := "http://squid.myproxy.com"
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
		config := BackplaneConfiguration{URL: "https://api-backplane.apps.openshiftapps.com", ProxyURL: nil}
		err := config.CheckAPIConnection()

		if err != nil {
			t.Failed()
		}
	})
}
