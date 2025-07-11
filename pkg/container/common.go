package container

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"github.com/openshift/backplane-cli/pkg/ocm"
)

// fetchPullSecretIfNotExist will check if there's a pull secrect file
// under $HOME/.kube/, if not, it will ask OCM for the pull secrect
// The pull secret is written to a file
func fetchPullSecretIfNotExist() (string, string, error) {
	configDirectory, err := GetConfigDirectory()
	if err != nil {
		return "", "", err
	}

	configFilename := filepath.Join(configDirectory, "config.json")

	// Check if file already exists
	if _, err = os.Stat(configFilename); !os.IsNotExist(err) {
		return configDirectory, configFilename, nil
	}

	// If directory doesn't exist, create it with the right permissions
	if err := os.MkdirAll(configDirectory, 0700); err != nil {
		return "", "", err
	}

	response, err := ocm.DefaultOCMInterface.GetPullSecret()
	if err != nil {
		return "", "", fmt.Errorf("failed to get pull secret from ocm: %v", err)
	}
	err = os.WriteFile(configFilename, []byte(response), 0600)
	if err != nil {
		return "", "", fmt.Errorf("failed to write authfile for pull secret: %v", err)
	}

	return configDirectory, configFilename, nil
}

// GetConfigDirectory returns pull secret file saving path
// Defaults to ~/.kube/ocm-pull-secret
func GetConfigDirectory() (string, error) {
	if pullSecretConfigDirectory == "" {
		home, err := homedir.Dir()
		if err != nil {
			return "", fmt.Errorf("can't get user homedir. Error: %s", err.Error())
		}

		// Update config directory default path
		pullSecretConfigDirectory = filepath.Join(home, ".kube/ocm-pull-secret")
	}

	return pullSecretConfigDirectory, nil
}

// podman/docker container stop
// this action is OS independent
func generalStopContainer(containerEngine string, containerName string) error {
	engStopArgs := []string{
		"container",
		"stop",
		containerName,
	}
	stopCmd := createCommand(containerEngine, engStopArgs...)
	stopCmd.Stderr = os.Stderr
	stopCmd.Stdout = nil

	err := stopCmd.Run()

	if err != nil {
		return fmt.Errorf("failed to stop container %s: %s", containerName, err)
	}
	return nil
}

func generalContainerIsExist(containerEngine string, containerName string) (bool, error) {
	var out bytes.Buffer
	filter := fmt.Sprintf("name=%s", containerName)
	existArgs := []string{
		"ps",
		"-aq",
		"--filter",
		filter,
	}
	existCmd := createCommand(containerEngine, existArgs...)
	existCmd.Stderr = os.Stderr
	existCmd.Stdout = &out

	err := existCmd.Run()

	if err != nil {
		return false, fmt.Errorf("failed to check container exist %s: %s", containerName, err)
	}
	if out.String() != "" {
		return true, nil
	}
	return false, nil
}
