package container

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openshift/backplane-cli/pkg/cli/config"
	logger "github.com/sirupsen/logrus"
)

type dockerLinux struct{}
type dockerMac struct{}

// docker pull - pull the image to local from registry
// this action is OS independent
func generalDockerPullImage(imageName string) error {
	// Ensure we have authfile to pull image
	configDirectory, _, err := fetchPullSecretIfNotExist()
	if err != nil {
		return err
	}
	engPullArgs := []string{
		"--config", configDirectory, // in docker, --config should be made first
		"pull",
		"--quiet",
		"--platform=linux/amd64", // always run linux/amd64 image
		imageName,
	}
	logger.WithField("Command", fmt.Sprintf("`%s %s`", DOCKER, strings.Join(engPullArgs, " "))).Infoln("Pulling image")
	pullCmd := createCommand(DOCKER, engPullArgs...)
	pullCmd.Stderr = os.Stderr
	pullCmd.Stdout = nil
	err = pullCmd.Run()
	if err != nil {
		return err
	}
	return nil
}

// docker-pull for Linux
func (ce *dockerLinux) PullImage(imageName string) error {
	return generalDockerPullImage(imageName)
}

// docker-pull for Mac
func (ce *dockerMac) PullImage(imageName string) error {
	return generalDockerPullImage(imageName)
}

// the shared function for docker to run console container for both linux and macOS
func dockerRunConsoleContainer(containerName string, port string, consoleArgs []string, envVars []EnvVar) error {
	configDirectory, _, err := fetchPullSecretIfNotExist()
	if err != nil {
		return err
	}
	// For docker on linux, we need to use host network,
	// otherwise it won't go through the sshuttle.
	// TODO: confirm if that's the case with new backplane setup
	engRunArgs := []string{
		"--config", configDirectory, // in docker, --config should be made first
		"run",
		"--platform=linux/amd64", // always run linux/amd64 image
		"--rm",
		"--detach", // run in background
		"--name", containerName,
		"--publish", fmt.Sprintf("127.0.0.1:%s:%s", port, port),
		"--network", "host",
	}
	for _, e := range envVars {
		engRunArgs = append(engRunArgs,
			"--env", fmt.Sprintf("%s=%s", e.Key, e.Value),
		)
	}
	engRunArgs = append(engRunArgs, consoleArgs...)
	logger.WithField("Command", fmt.Sprintf("`%s %s`", DOCKER, strings.Join(engRunArgs, " "))).Infoln("Running container")

	runCmd := createCommand(DOCKER, engRunArgs...)
	runCmd.Stderr = os.Stderr
	runCmd.Stdout = nil

	return runCmd.Run()
}

func (ce *dockerMac) RunConsoleContainer(containerName string, port string, consoleArgs []string, envVars []EnvVar) error {
	return dockerRunConsoleContainer(containerName, port, consoleArgs, envVars)
}

func (ce *dockerLinux) RunConsoleContainer(containerName string, port string, consoleArgs []string, envVars []EnvVar) error {
	return dockerRunConsoleContainer(containerName, port, consoleArgs, envVars)
}

// the shared function for docker to run monitoring plugin for both linux and macOS
func dockerRunMonitorPlugin(containerName string,
	_ string,
	nginxConfPath string,
	pluginArgs []string,
	envVars []EnvVar,
) error {
	configDirectory, _, err := fetchPullSecretIfNotExist()
	if err != nil {
		return err
	}
	engRunArgs := []string{
		"--config", configDirectory, // in docker, --config should be made first
		"run",
		"--platform=linux/amd64", // always run linux/amd64 image
		"--rm",
		"--detach", // run in background
		"--name", containerName,
		"--network", "host",
	}

	// nginxConfPath is optional. Add --volume when the nginxConfPath is not empty.
	if nginxConfPath != "" {
		volArg := fmt.Sprintf("%s:/etc/nginx/nginx.conf:z", nginxConfPath)
		engRunArgs = append(engRunArgs, "--volume", volArg)
	}

	for _, e := range envVars {
		engRunArgs = append(engRunArgs,
			"--env", fmt.Sprintf("%s=%s", e.Key, e.Value),
		)
	}

	engRunArgs = append(engRunArgs, pluginArgs...)

	logger.WithField("Command", fmt.Sprintf("`%s %s`", DOCKER, strings.Join(engRunArgs, " "))).Infoln("Running container")
	runCmd := createCommand(DOCKER, engRunArgs...)
	runCmd.Stderr = os.Stderr
	runCmd.Stdout = nil

	return runCmd.Run()
}

func (ce *dockerLinux) RunMonitorPlugin(containerName string, consoleContainerName string, nginxConf string, pluginArgs []string, envVars []EnvVar) error {
	var nginxConfPath string
	if nginxConf != "" {
		configDirectory, err := config.GetConfigDirectory()
		if err != nil {
			return err
		}
		nginxConfPath = filepath.Join(configDirectory, nginxConf)
	}

	return dockerRunMonitorPlugin(containerName, consoleContainerName, nginxConfPath, pluginArgs, envVars)
}

func (ce *dockerMac) RunMonitorPlugin(containerName string, consoleContainerName string, nginxConf string, pluginArgs []string, envVars []EnvVar) error {
	var nginxConfPath string
	if nginxConf != "" {
		configDirectory, err := config.GetConfigDirectory()
		if err != nil {
			return err
		}
		nginxConfPath = filepath.Join(configDirectory, nginxConf)
	}

	return dockerRunMonitorPlugin(containerName, consoleContainerName, nginxConfPath, pluginArgs, envVars)
}

// put a file in place for container to mount
// filename should be name only, not a path
func dockerPutFileToMount(filename string, content []byte) error {
	// for files in linux, we put them into the user's backplane config directory
	configDirectory, err := config.GetConfigDirectory()
	if err != nil {
		return err
	}
	dstFileName := filepath.Join(configDirectory, filename)

	// Check if file already exists, if it does remove it
	if _, err = os.Stat(dstFileName); !os.IsNotExist(err) {
		logger.Debugf("remove existing file %s", dstFileName)
		err = os.Remove(dstFileName)
		if err != nil {
			return err
		}
	}

	if err = os.WriteFile(dstFileName, content, 0600); err != nil {
		logger.Debugf("wrote file %s", dstFileName)
		return err
	}

	// change permission as a work around to gosec
	if err = os.Chmod(dstFileName, 0644); err != nil {
		logger.Debugf("change permission to 0644 for %s", dstFileName)
		return err
	}

	return nil
}

func (ce *dockerLinux) PutFileToMount(filename string, content []byte) error {
	return dockerPutFileToMount(filename, content)
}

func (ce *dockerMac) PutFileToMount(filename string, content []byte) error {
	return dockerPutFileToMount(filename, content)
}

// docker-stop for Linux
func (ce *dockerLinux) StopContainer(containerName string) error {
	return generalStopContainer(DOCKER, containerName)
}

// docker-stop for macOS
func (ce *dockerMac) StopContainer(containerName string) error {
	return generalStopContainer(DOCKER, containerName)
}

// docker-exist for Linux
func (ce *dockerLinux) ContainerIsExist(containerName string) (bool, error) {
	return generalContainerIsExist(DOCKER, containerName)
}

// docker-exist for macOS
func (ce *dockerMac) ContainerIsExist(containerName string) (bool, error) {
	return generalContainerIsExist(DOCKER, containerName)
}
