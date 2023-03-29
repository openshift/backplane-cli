package config

import (
	"testing"

	"github.com/openshift/backplane-cli/pkg/info"
)

func TestGetBackplaneConfigFile(t *testing.T) {
	t.Run("it returns the Backplane configuration file path if it exists in the user's env", func(t *testing.T) {
		t.Setenv(info.BACKPLANE_CONFIG_PATH_ENV_NAME, "~/.backplane.stg.env.json")
		path, err := GetBackplaneConfigFile()
		if err != nil {
			t.Error(err)
		}
		if path != "~/.backplane.stg.env.json" {
			t.Errorf("expected path to be %v, got %v", "~/.backplane.stg.env.json", path)
		}
	})

	t.Run("it returns the default configuration file path if it does not exist in the user's env", func(t *testing.T) {
		path, err := GetBackplaneConfigFile()
		if err != nil {
			t.Error(err)
		}

		expectedPath, err := getDefaultConfigPath()
		if err != nil {
			t.Error(err)
		}
		if path != expectedPath {
			t.Errorf("expected path to be %v, got %v", expectedPath, path)
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
