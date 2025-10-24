package container

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	logger "github.com/sirupsen/logrus"
)

type podmanLinux struct {
	fileMountDir string
}
type podmanMac struct{}

// podman pull - pull the image to local from registry
// this action is OS independent
func generalPodmanPullImage(imageName string) error {
	// Ensure we have authfile to pull image
	_, configFilename, err := fetchPullSecretIfNotExist()
	if err != nil {
		return err
	}
	engPullArgs := []string{
		"pull",
		"--quiet",
		"--authfile", configFilename,
		"--platform=linux/amd64", // always run linux/amd64 image
		imageName,
	}
	logger.WithField("Command", fmt.Sprintf("`%s %s`", PODMAN, strings.Join(engPullArgs, " "))).Infoln("Pulling image")
	pullCmd := createCommand(PODMAN, engPullArgs...)
	pullCmd.Stderr = os.Stderr
	pullCmd.Stdout = nil
	err = pullCmd.Run()
	if err != nil {
		return err
	}
	return nil
}

// podman-pull for Linux
func (ce *podmanLinux) PullImage(imageName string) error {
	return generalPodmanPullImage(imageName)
}

// podman-pull for Mac
func (ce *podmanMac) PullImage(imageName string) error {
	return generalPodmanPullImage(imageName)
}

// the shared function for podman to run console container for both linux and macOS
func podmanRunConsoleContainer(containerName string, port string, consoleArgs []string, envVars []EnvVar) error {
	_, authFilename, err := fetchPullSecretIfNotExist()
	if err != nil {
		return err
	}
	engRunArgs := []string{
		"run",
		"--authfile", authFilename,
		"--platform=linux/amd64", // always run linux/amd64 image
		"--rm",
		"--detach", // run in background
		"--name", containerName,
		"--publish", fmt.Sprintf("127.0.0.1:%s:%s", port, port),
	}
	for _, e := range envVars {
		engRunArgs = append(engRunArgs,
			"--env", fmt.Sprintf("%s=%s", e.Key, e.Value),
		)
	}
	engRunArgs = append(engRunArgs, consoleArgs...)
	logger.WithField("Command", fmt.Sprintf("`%s %s`", PODMAN, strings.Join(engRunArgs, " "))).Infoln("Running container")

	runCmd := createCommand(PODMAN, engRunArgs...)
	runCmd.Stderr = os.Stderr
	runCmd.Stdout = nil

	return runCmd.Run()
}

func (ce *podmanMac) RunConsoleContainer(containerName string, port string, consoleArgs []string, envVars []EnvVar) error {
	return podmanRunConsoleContainer(containerName, port, consoleArgs, envVars)
}

func (ce *podmanLinux) RunConsoleContainer(containerName string, port string, consoleArgs []string, envVars []EnvVar) error {
	return podmanRunConsoleContainer(containerName, port, consoleArgs, envVars)
}

// the shared function for podman to run monitoring plugin for both linux and macOS
func podmanRunMonitorPlugin(
	containerName string,
	consoleContainerName string,
	nginxConfPath string,
	pluginArgs []string,
	envVars []EnvVar,
) error {
	_, authFilename, err := fetchPullSecretIfNotExist()
	if err != nil {
		return err
	}
	engRunArgs := []string{
		"run",
		"--authfile", authFilename,
		"--platform=linux/amd64", // always run linux/amd64 image
		"--rm",
		"--detach", // run in background
		"--name", containerName,
		"--network", fmt.Sprintf("container:%s", consoleContainerName),
	}

	// nginxConfPath is optional. Add --mount when the nginxConfPath is not empty.
	if nginxConfPath != "" {
		mountArg := fmt.Sprintf("type=bind,source=%s,destination=/etc/nginx/nginx.conf,relabel=shared", nginxConfPath)
		engRunArgs = append(engRunArgs, "--mount", mountArg)
	}

	for _, e := range envVars {
		engRunArgs = append(engRunArgs,
			"--env", fmt.Sprintf("%s=%s", e.Key, e.Value),
		)
	}

	engRunArgs = append(engRunArgs, pluginArgs...)

	logger.WithField("Command", fmt.Sprintf("`%s %s`", PODMAN, strings.Join(engRunArgs, " "))).Infoln("Running container")
	runCmd := createCommand(PODMAN, engRunArgs...)
	runCmd.Stderr = os.Stderr
	runCmd.Stdout = nil

	return runCmd.Run()
}

func (ce *podmanMac) RunMonitorPlugin(containerName string, consoleContainerName string, nginxConf string, pluginArgs []string, envVars []EnvVar) error {
	var nginxConfPath string
	if nginxConf != "" {
		nginxConfPath = filepath.Join("/tmp/", nginxConf)
	}
	return podmanRunMonitorPlugin(containerName, consoleContainerName, nginxConfPath, pluginArgs, envVars)
}

func (ce *podmanLinux) RunMonitorPlugin(containerName string, consoleContainerName string, nginxConf string, pluginArgs []string, envVars []EnvVar) error {
	var nginxConfPath string
	if nginxConf != "" {
		nginxConfPath = filepath.Join(ce.fileMountDir, nginxConf)
	}
	return podmanRunMonitorPlugin(containerName, consoleContainerName, nginxConfPath, pluginArgs, envVars)
}

// put a file in place for container to mount
// filename should be name only, not a path
func (ce *podmanLinux) PutFileToMount(filename string, content []byte) error {
	// ensure the directory exists
	err := os.MkdirAll(ce.fileMountDir, os.ModePerm) //nolint:gosec
	if err != nil {
		return err
	}
	dstFileName := filepath.Join(ce.fileMountDir, filename)

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
	if err = os.Chmod(dstFileName, 0640); err != nil { //nolint:gosec
		logger.Debugf("change permission to 0640 for %s", dstFileName)
		return err
	}

	return nil
}

// put a file in place for container to mount
// filename should be name only, not a path
func (ce *podmanMac) PutFileToMount(filename string, content []byte) error {
	// Podman in Mac runs on a VM, we need to put the file to the VM.
	// To do so, we encode the content, send it to VM via ssh then decode it.
	dstFilename := filepath.Join("/tmp/", filename)
	contentEncoded := base64.StdEncoding.EncodeToString(content)
	writeConfigCmd := fmt.Sprintf("podman machine ssh $(podman machine info --format {{.Host.CurrentMachine}}) \"echo %s | base64 -d > %s \"", contentEncoded, dstFilename)
	logger.Debugf("Executing: %s\n", writeConfigCmd)
	writeConfigOutput, err := createCommand("bash", "-c", writeConfigCmd).CombinedOutput()
	if err != nil {
		return err
	}
	logger.Debugln(writeConfigOutput)
	return nil
}

// podman-stop for Linux
func (ce *podmanLinux) StopContainer(containerName string) error {
	return generalStopContainer(PODMAN, containerName)
}

// podman-stop for macOS
func (ce *podmanMac) StopContainer(containerName string) error {
	return generalStopContainer(PODMAN, containerName)
}

// podman-exist for Linux
func (ce *podmanLinux) ContainerIsExist(containerName string) (bool, error) {
	return generalContainerIsExist(PODMAN, containerName)
}

// podman-exist for macOS
func (ce *podmanMac) ContainerIsExist(containerName string) (bool, error) {
	return generalContainerIsExist(PODMAN, containerName)
}
