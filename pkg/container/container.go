package container

import "os/exec"

const (
	// DOCKER binary name of docker
	DOCKER = "docker"
	// PODMAN binary name of podman
	PODMAN = "podman"
	// Linux name in runtime.GOOS
	LINUX = "linux"
	// MACOS name in runtime.GOOS
	MACOS = "darwin"
)

var (
	createCommand = exec.Command
	// Pull Secret saving directory
	pullSecretConfigDirectory string
)

type ContainerEngine interface {
	PullImage(imageName string) error
	PutFileToMount(filename string, content []byte) error
	StopContainer(containerName string) error
	RunConsoleContainer(containerName string, port string, consoleArgs []string, envVars []EnvVar) error
	RunMonitorPlugin(containerName string, consoleContainerName string, nginxConf string, pluginArgs []string, envVars []EnvVar) error
	ContainerIsExist(containerName string) (bool, error)
}

// EnvVar for environment variable passing to container
type EnvVar struct {
	Key   string
	Value string
}
