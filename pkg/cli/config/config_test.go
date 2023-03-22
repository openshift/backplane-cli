package config

import (
	"testing"

	"github.com/openshift/backplane-cli/pkg/info"
)

func TestGetBackplaneConfigFile(t *testing.T) {
	t.Run("it returns the Backplane configuration file path if it exists in the user's env", func(t *testing.T) {
		t.Setenv(info.BACKPLANE_CONFIG_PATH_ENV_NAME, "~/.backplane.stg.env.json")
		path := GetBackplaneConfigFile()
		if path != "~/.backplane.stg.env.json" {
			t.Errorf("expected path to be %v, got %v", "~/.backplane.stg.env.json", path)
		}
	})

	t.Run("it returns the default configuration file path if it does not exist in the user's env", func(t *testing.T) {
		path := GetBackplaneConfigFile()
		expectedPath := getConfiDefaultPath(info.BACKPLANE_CONFIG_DEFAULT_FILE_NAME)
		if path != expectedPath {
			t.Errorf("expected path to be %v, got %v", expectedPath, path)
		}
	})
}
