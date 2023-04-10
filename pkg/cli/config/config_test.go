package config

import (
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

		if config.ProxyURL != userDefinedProxy {
			t.Errorf("expected to return the explicitly defined proxy %v instead of the default one %v", userDefinedProxy, config.ProxyURL)
		}
	})
}

func TestGetBackplaneConfiguration(t *testing.T) {

	for name, tc := range map[string]struct {
		envNeedToSet         bool
		backplaneURLEnvVar   string
		proxyUrl             string
		expectedBackplaneURL string
		expectedError        bool
	}{
		"backplane url set via env vars": {
			envNeedToSet:         true,
			backplaneURLEnvVar:   "https://api-backplane.apps.openshiftapps.com",
			proxyUrl:             "http://squid.myproxy.com",
			expectedBackplaneURL: "https://api-backplane.apps.openshiftapps.com",
			expectedError:        false,
		},
		"backplane url set empty env vars": {
			envNeedToSet:         true,
			proxyUrl:             "",
			backplaneURLEnvVar:   "",
			expectedBackplaneURL: "",
			expectedError:        true,
		},
	} {
		tc := tc

		t.Run(name, func(t *testing.T) {
			if tc.envNeedToSet {
				t.Setenv("BACKPLANE_URL", tc.backplaneURLEnvVar)
				t.Setenv("HTTPS_PROXY", tc.proxyUrl)
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
