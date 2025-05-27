/*
Copyright © 2021 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package console

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Masterminds/semver"
	"github.com/mitchellh/go-homedir"
	consolev1typedclient "github.com/openshift/client-go/console/clientset/versioned/typed/console/v1"
	consolev1alpha1typedclient "github.com/openshift/client-go/console/clientset/versioned/typed/console/v1alpha1"
	operatorv1typedclient "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	"github.com/pkg/browser"
	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/info"
	"github.com/openshift/backplane-cli/pkg/ocm"
	"github.com/openshift/backplane-cli/pkg/utils"
)

type containerEngineInterface interface {
	pullImage(imageName string) error
	putFileToMount(filename string, content []byte) error
	stopContainer(containerName string) error
	runConsoleContainer(containerName string, port string, consoleArgs []string, envVars []envVar) error
	runMonitorPlugin(containerName string, consoleContainerName string, nginxConf string, pluginArgs []string) error
	containerIsExist(containerName string) (bool, error)
}

type podmanLinux struct {
	fileMountDir string
}
type podmanMac struct{}
type dockerLinux struct{}
type dockerMac struct{}

type execActionOnTermInterface interface {
	execActionOnTerminationFunction(action postTerminateFunc) error
}

type execActionOnTermStruct struct{}

type consoleOptions struct {
	image               string
	port                string
	consoleHost         string
	containerEngineFlag string
	url                 string
	openBrowser         bool
	enablePlugins       bool
	needMonitorPlugin   bool
	monitorPluginPort   string
	monitorHost         string
	monitorPluginImage  string
	terminationFunction execActionOnTermInterface
}

// envVar for environment variable passing to container
type envVar struct {
	key   string
	value string
}

const (
	// DOCKER binary name of docker
	DOCKER = "docker"
	// PODMAN binary name of podman
	PODMAN = "podman"
	// Linux name in runtime.GOOS
	LINUX = "linux"
	// MACOS name in runtime.GOOS
	MACOS = "darwin"

	// Environment variable that indicates if open by browser is set as default
	EnvBrowserDefault = "BACKPLANE_DEFAULT_OPEN_BROWSER"

	// Environment variable that set the container engine
	EnvContainerEngine = "CONTAINER_ENGINE"

	// Minimum required version for monitoring-plugin container
	versionForMonitoringPlugin = "4.14"

	// Minimum required version to use backend service for plugins
	versionForConsolePluginsBackendService = "4.12"

	// The namespace where console deploys in the cluster
	ConsoleNS = "openshift-console"

	// The deployment name of console
	ConsoleDeployment = "console"

	// The namespace of monitoring stack
	MonitoringNS = "openshift-monitoring"

	// The deployment name of monitoring plugin
	MonitoringPluginDeployment = "monitoring-plugin"
)

var (
	validContainerEngines = []string{PODMAN, DOCKER}
	// For mocking
	createClientSet = func(c *rest.Config) (kubernetes.Interface, error) { return kubernetes.NewForConfig(c) }
	createCommand   = exec.Command

	// Pull Secret saving directory
	pullSecretConfigDirectory string
)

func newConsoleOptions() *consoleOptions {
	return &consoleOptions{
		terminationFunction: &execActionOnTermStruct{},
	}
}

func NewConsoleCmd() *cobra.Command {
	ops := newConsoleOptions()
	consoleCmd := &cobra.Command{
		Use:   "console",
		Short: "Launch OpenShift web console for the current cluster",
		Long: `console will start the Openshift console application locally for the currently logged in cluster.
Default behaviour is to run the same console image as the cluster.
Clusters below 4.8 will not display metrics, alerts, or dashboards. If you need to view metrics, alerts, or dashboards use the latest console image
with --image=quay.io/openshift/origin-console .
You can specify container engine with -c. If not specified, it will lookup the PATH in the order of podman and docker.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ops.run()
		},
	}

	flags := consoleCmd.Flags()
	flags.StringVar(
		&ops.image,
		"image",
		"",
		"Specify console image to use. Default: The same console image in the cluster",
	)
	flags.StringVar(
		&ops.port,
		"console-port",
		"",
		"Specify port for console. Default: A random available port",
	)
	flags.StringVar(
		&ops.consoleHost,
		"console-host",
		"127.0.0.1",
		"Specify host for console. Default: 127.0.0.1",
	)
	flags.StringVar(
		&ops.monitorHost,
		"monitor-host",
		"127.0.0.1",
		"Specify host for monitoring plugin. Default: 127.0.0.1",
	)
	flags.StringVar(
		&ops.monitorPluginPort,
		"monitor-port",
		"",
		"Specify port for monitoring plugin. Default: A random available port",
	)
	flags.BoolVarP(
		&ops.openBrowser,
		"browser",
		"b",
		false,
		fmt.Sprintf("Open a browser after the console container starts. Can also be set via the environment variable '%s'", EnvBrowserDefault),
	)
	flags.StringVarP(
		&ops.containerEngineFlag,
		"container-engine",
		"c",
		"",
		fmt.Sprintf("Specify container engine. -c %s.", strings.Join(validContainerEngines, "|")),
	)
	flags.BoolVarP(
		&ops.enablePlugins,
		"plugins",
		"",
		false,
		"Load enabled dynamic console plugins on the cluster. Default: false",
	)
	flags.StringVarP(
		&ops.url,
		"url",
		"u",
		"",
		"The full console url, e.g. from PagerDuty. The hostname will be replaced with that of the locally running console.",
	)

	return consoleCmd
}

func (o *consoleOptions) run() error {
	err := o.determineOpenBrowser()
	if err != nil {
		return err
	}
	ce, err := o.getContainerEngineImpl()
	if err != nil {
		return err
	}
	err = o.determineListenPort()
	if err != nil {
		return err
	}
	kubeconfig, err := getCurrentKubeconfig()
	if err != nil {
		return err
	}
	err = o.determineImage(kubeconfig)
	if err != nil {
		return err
	}
	// pull the console image
	err = o.pullConsoleImage(ce)
	if err != nil {
		return err
	}
	err = o.determineNeedMonitorPlugin()
	if err != nil {
		return err
	}
	err = o.determineMonitorPluginPort()
	if err != nil {
		return err
	}
	err = o.determineMonitorPluginImage(kubeconfig)
	if err != nil {
		return err
	}
	// pull the monitoring plugin image
	err = o.pullMonitorPluginImage(ce)
	if err != nil {
		return err
	}
	// Perform a cleanup before starting a new console
	err = o.beforeStartCleanUp(ce)
	if err != nil {
		return err
	}

	errs := make(chan error)
	go o.runContainers(ce, errs)
	if len(errs) != 0 {
		err := <-errs
		return err
	}

	err = o.cleanUp(ce)
	if err != nil {
		return err
	}

	return nil
}

func (o *consoleOptions) runContainers(ce containerEngineInterface, errs chan<- error) {
	if err := o.runConsoleContainer(ce); err != nil {
		errs <- err
		return
	}

	if err := o.runMonitorPlugin(ce); err != nil {
		errs <- err
	}

	if err := o.printURL(); err != nil {
		errs <- err
	}
}

// Parse environment variables
func (o *consoleOptions) determineOpenBrowser() error {
	if env, ok := os.LookupEnv(EnvBrowserDefault); ok {
		// if set, try to parse it as a bool and pass it into consoleOptions.openBrowser
		openBrowser, err := strconv.ParseBool(env)
		if err != nil {
			return fmt.Errorf("unable to parse boolean value from environment variable %v", EnvBrowserDefault)
		}
		o.openBrowser = openBrowser
	}
	logger.Debugf("auto open browser: %t\n", o.openBrowser)
	return nil
}

// check if the container engine is supported and exists in PATH
func validateContainerEngine(containerEngine string) (valid bool, err error) {
	if len(containerEngine) == 0 {
		return false, fmt.Errorf("container engine should not be blank")
	}
	matchedEngine := ""
	for _, ce := range validContainerEngines {
		if strings.EqualFold(containerEngine, ce) {
			matchedEngine = ce
		}
	}
	if len(matchedEngine) == 0 {
		return false, fmt.Errorf("container engine can only be one of %s", strings.Join(validContainerEngines, "|"))
	}

	if _, err := exec.LookPath(containerEngine); err != nil {
		return false, fmt.Errorf("cannot find %s in PATH", containerEngine)
	}

	return true, nil
}

func (o *consoleOptions) getContainerEngineImpl() (containerEngineInterface, error) {
	// Pick a container engine implementation.
	// If user specify by -c, use it;
	// else check if user specify by environment variable;
	// else lookup from PATH;
	// Then, confirm the engine is valid
	containerEngine := ""
	if len(o.containerEngineFlag) > 0 {
		// get from flag
		containerEngine = o.containerEngineFlag
		logger.Debugf("container engine specified in flag: %s\n", containerEngine)
	} else if engine, hasEngine := os.LookupEnv(EnvContainerEngine); hasEngine {
		// get from env
		containerEngine = engine
		logger.Debugf("container engine specified in environment: %s\n", containerEngine)
	} else {
		// lookup from PATH
		for _, ce := range validContainerEngines {
			if _, err := exec.LookPath(ce); err == nil {
				containerEngine = ce
				break
			}
		}
		if len(containerEngine) == 0 {
			return nil, fmt.Errorf("can't find %s in PATH, please install one of the container engines", strings.Join(validContainerEngines, "|"))
		}
		logger.Debugf("container engine found in path: %s\n", containerEngine)
	}

	// confirm if it exist in path
	valid, err := validateContainerEngine(containerEngine)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, fmt.Errorf("failed to validate container engine: %s", containerEngine)
	}

	logger.Infof("Using container engine %s\n", containerEngine)

	var containerEngineImpl containerEngineInterface
	if runtime.GOOS == LINUX && containerEngine == PODMAN {
		containerEngineImpl = &podmanLinux{fileMountDir: filepath.Join(os.TempDir(), "backplane")}
	} else if runtime.GOOS == MACOS && containerEngine == PODMAN {
		containerEngineImpl = &podmanMac{}
	} else if runtime.GOOS == LINUX && containerEngine == DOCKER {
		containerEngineImpl = &dockerLinux{}
	} else if runtime.GOOS == MACOS && containerEngine == DOCKER {
		containerEngineImpl = &dockerMac{}
	}
	return containerEngineImpl, nil
}

// determine a port for the console container to expose
// if not specified, pick a random port
func (o *consoleOptions) determineListenPort() error {
	if len(o.port) > 0 {
		_, err := strconv.Atoi(o.port)
		if err != nil {
			return fmt.Errorf("port should be an integer")
		}
	} else {
		port, err := utils.GetFreePort()
		if err != nil {
			return fmt.Errorf("failed looking up a free port: %s", err)
		}
		o.port = strconv.Itoa(port)
	}
	logger.Debugf("using listen port: %s\n", o.port)
	return nil
}

func (o *consoleOptions) determineImage(config *rest.Config) error {
	// Get image
	if len(o.image) == 0 {
		logger.Debugln("Querying the cluster for console image")
		image, err := getConsoleImageFromCluster(config)
		if err != nil {
			return err
		}
		o.image = image
	}
	logger.Infof("Using image %s\n", o.image)
	return nil
}

func (o *consoleOptions) pullConsoleImage(ce containerEngineInterface) error {
	return ce.pullImage(o.image)
}

func (o *consoleOptions) determineNeedMonitorPlugin() error {
	if isRunningHigherOrEqualTo(versionForMonitoringPlugin) {
		logger.Debugln("monitoring plugin is needed")
		o.needMonitorPlugin = true
		return nil
	} else {
		logger.Debugln("monitoring plugin is not needed")
		o.needMonitorPlugin = false
		return nil
	}
}

func (o *consoleOptions) determineMonitorPluginPort() error {
	if !o.needMonitorPlugin {
		logger.Debugln("monitoring plugin is not needed, not to assign monitoring plugin port")
		return nil
	}
	port, err := utils.GetFreePort()
	if err != nil {
		return fmt.Errorf("failed looking up a free port for monitoring plugin: %s", err)
	}
	o.monitorPluginPort = strconv.Itoa(port)
	logger.Debugf("using monitoring plugin port: %s\n", o.monitorPluginPort)
	return nil
}

func (o *consoleOptions) determineMonitorPluginImage(config *rest.Config) error {
	// we don't need this for lower than 4.14
	if !o.needMonitorPlugin {
		logger.Debugln("monitoring plugin is not needed, not to get monitoring plugin image")
		return nil
	}
	// Get monitoring image
	if len(o.monitorPluginImage) == 0 {
		logger.Debugln("Querying the cluster for monitoring plugin image")
		image, err := getMonitoringPluginImageFromCluster(config)
		if err != nil {
			return err
		}
		o.monitorPluginImage = image
	}
	logger.Infof("Using monitoring plugin image %s\n", o.monitorPluginImage)
	return nil
}

func (o *consoleOptions) pullMonitorPluginImage(ce containerEngineInterface) error {
	if !o.needMonitorPlugin {
		logger.Debugln("monitoring plugin is not needed, not to pull monitoring plugin image")
		return nil
	}
	return ce.pullImage(o.monitorPluginImage)
}

// return a list of plugins for console to load, including the monitoring plugin
// eg, "plugin-name=plugin-endpoint plugin-name2=plugin-endpoint2"
// https://github.com/openshift/console/blob/0ed60b588f0be2090f3bec5a6a4c4e67eb8dc1ef/pkg/serverconfig/types.go#L23
// https://github.com/openshift/console/blob/0ed60b588f0be2090f3bec5a6a4c4e67eb8dc1ef/pkg/serverconfig/config.go#L20
func (o *consoleOptions) getPlugins() (string, error) {
	var plugins []string
	var consolePlugins []string

	// user enabled plugins
	if o.enablePlugins {
		config, err := getCurrentKubeconfig()
		if err != nil {
			return "", err
		}
		if isRunningHigherOrEqualTo(versionForConsolePluginsBackendService) {
			consolePlugins, err = getConsolePluginFromCluster(config)
			if err != nil {
				return "", err
			}
		} else {
			consolePlugins, err = getConsolePluginFrom411Cluster(config)
			if err != nil {
				return "", err
			}
		}
		plugins = append(plugins, consolePlugins...)
	}
	// monitoring plugin
	if o.needMonitorPlugin {
		logger.Debugln("monitoring plugin is needed, adding the monitoring plugin parameter to console container")
		plugins = append(plugins, fmt.Sprintf("monitoring-plugin=http://%s:%s", o.monitorHost, o.monitorPluginPort))
	}

	return strings.Join(plugins, ","), nil
}

func (o *consoleOptions) runConsoleContainer(ce containerEngineInterface) error {
	clusterID, err := getClusterID()
	if err != nil {
		return err
	}
	consoleContainerName := fmt.Sprintf("console-%s", clusterID)

	c, err := ocm.DefaultOCMInterface.GetClusterInfoByID(clusterID)
	if err != nil {
		return err
	}
	p, ok := c.GetProduct()
	if !ok {
		return fmt.Errorf("could not get product information")
	}

	branding := "dedicated"
	documentationURL := "https://docs.openshift.com/dedicated/4/"
	if p.ID() == "rosa" {
		branding = "ocp"
		documentationURL = "https://docs.openshift.com/rosa/"
	}

	// Get the RESTconfig from the current kubeconfig context.
	config, err := getCurrentKubeconfig()
	if err != nil {
		return err
	}

	// Construct endpoint locations.
	apiURL := config.Host // == currentClusterInfo.ClusterURL
	logger.Debugf("API endpoint: %s\n", apiURL)
	if !strings.Contains(apiURL, "backplane/cluster") {
		return fmt.Errorf("the api server is not a backplane url, please make sure you login the cluster using backplane")
	}
	alertmanagerURL := strings.Replace(apiURL, "backplane/cluster", "backplane/alertmanager", 1)
	alertmanagerURL = strings.TrimSuffix(alertmanagerURL, "/")
	thanosURL := strings.Replace(apiURL, "backplane/cluster", "backplane/thanos", 1)
	thanosURL = strings.TrimSuffix(thanosURL, "/")

	// Get ocm access token
	logger.Debugln("Finding ocm token")
	ocmToken, err := ocm.DefaultOCMInterface.GetOCMAccessToken()
	if err != nil {
		return err
	}

	bridgeListen := fmt.Sprintf("http://%s:%s", o.consoleHost, o.port)

	var envVars []envVar
	// Set proxy URL to the container
	proxyURL, err := getProxyURL()
	if err != nil {
		return err
	}
	if proxyURL != nil {
		envVars = append(envVars, envVar{key: "HTTPS_PROXY", value: *proxyURL})
	}

	// monitoring plugin and user plugins
	plugins, err := o.getPlugins()
	if err != nil {
		return fmt.Errorf("failed geting plugins: %v", err)
	}
	if len(plugins) > 0 {
		envVars = append(envVars, envVar{key: "BRIDGE_PLUGINS", value: plugins})
	}

	containerArgs := []string{
		o.image,
		"/opt/bridge/bin/bridge",
		"--public-dir=/opt/bridge/static",
		"-base-address", fmt.Sprintf("http://%s:%s", o.consoleHost, o.port),
		"-branding", branding,
		"-documentation-base-url", documentationURL,
		"-user-settings-location", "localstorage",
		"-user-auth", "disabled",
		"-k8s-mode", "off-cluster",
		"-k8s-auth", "bearer-token",
		"-k8s-mode-off-cluster-endpoint", apiURL,
		"-k8s-mode-off-cluster-alertmanager", alertmanagerURL,
		"-k8s-mode-off-cluster-thanos", thanosURL,
		"-k8s-auth-bearer-token", *ocmToken,
		"-listen", bridgeListen,
	}

	return ce.runConsoleContainer(consoleContainerName, o.port, containerArgs, envVars)
}

func podmanRunMonitorPluginDedicated(containerName string, nginxConfPath string, pluginArgs []string, host string, port string) error {
	_, authFilename, err := fetchPullSecretIfNotExist()
	if err != nil {
		return err
	}
	engRunArgs := []string{
		"run",
		"--authfile", authFilename,
		"--platform=linux/amd64",
		"--rm",
		"--detach",
		"--name", containerName,
		"--publish", fmt.Sprintf("%s:%s:%s", host, port, port), // Expose dedicated port
		"--mount", fmt.Sprintf("type=bind,source=%s,destination=/etc/nginx/nginx.conf,relabel=shared", nginxConfPath),
	}
	engRunArgs = append(engRunArgs, pluginArgs...)
	logger.WithField("Command", fmt.Sprintf("`%s %s`", PODMAN, strings.Join(engRunArgs, " "))).Infoln("Running container")
	runCmd := createCommand(PODMAN, engRunArgs...)
	runCmd.Stderr = os.Stderr
	runCmd.Stdout = nil
	return runCmd.Run()
}

func dockerRunMonitorPluginDedicated(containerName string, nginxConfPath string, pluginArgs []string, host string, port string) error {
	configDirectory, _, err := fetchPullSecretIfNotExist()
	if err != nil {
		return err
	}
	engRunArgs := []string{
		"--config", configDirectory,
		"run",
		"--platform=linux/amd64",
		"--rm",
		"--detach",
		"--name", containerName,
		"--publish", fmt.Sprintf("%s:%s:%s", host, port, port),
		"--volume", fmt.Sprintf("%s:/etc/nginx/nginx.conf:z", nginxConfPath),
	}
	engRunArgs = append(engRunArgs, pluginArgs...)
	logger.WithField("Command", fmt.Sprintf("`%s %s`", DOCKER, strings.Join(engRunArgs, " "))).Infoln("Running container")
	runCmd := createCommand(DOCKER, engRunArgs...)
	runCmd.Stderr = os.Stderr
	runCmd.Stdout = nil
	return runCmd.Run()
}

func (o *consoleOptions) runMonitorPlugin(ce containerEngineInterface) error {
	if !o.needMonitorPlugin {
		logger.Debugln("monitoring plugin is not needed, not to run monitoring plugin")
		return nil
	}

	clusterID, err := getClusterID()
	if err != nil {
		return err
	}

	// Setup nginx configurations
	configStr := fmt.Sprintf(info.MonitoringPluginNginxConfigTemplate, o.monitorPluginPort)
	nginxFilename := fmt.Sprintf(info.MonitoringPluginNginxConfigFilename, clusterID)
	if err := ce.putFileToMount(nginxFilename, []byte(configStr)); err != nil {
		return err
	}

	pluginContainerName := fmt.Sprintf("monitoring-plugin-%s", clusterID)

	pluginArgs := []string{o.monitorPluginImage}

	switch ce.(type) {
	case *podmanLinux, *podmanMac:
		nginxConfPath := filepath.Join("/tmp/", nginxFilename)
		return podmanRunMonitorPluginDedicated(pluginContainerName, nginxConfPath, pluginArgs, o.monitorHost, o.monitorPluginPort)
	case *dockerLinux, *dockerMac:
		configDir, err := config.GetConfigDirectory() // Correct usage
		if err != nil {
			return err
		}
		nginxConfPath := filepath.Join(configDir, nginxFilename)
		return dockerRunMonitorPluginDedicated(pluginContainerName, nginxConfPath, pluginArgs, o.monitorHost, o.monitorPluginPort)
	default:
		return fmt.Errorf("unsupported container engine type")
	}
}

// print the console URL and pop a browser if required
func (o *consoleOptions) printURL() error {
	// Store the locally running console URL or splice it into a url provided in consoleArgs.url
	consoleURL, err := replaceConsoleURL(fmt.Sprintf("http://127.0.0.1:%s", o.port), o.url)
	if err != nil {
		return fmt.Errorf("failed to replace url: %v", err)
	}

	fmt.Printf("== Console is available at %s ==\n\n", consoleURL)

	if o.openBrowser {
		go func() {
			err := wait.PollUntilContextTimeout(context.Background(), time.Second, 5*time.Second, true, func(context.Context) (bool, error) {
				return utils.CheckHealth(fmt.Sprintf("%s/health", consoleURL)), nil
			})
			if err != nil {
				logger.Warnf("failed waiting for container to become ready: %s", err)
				return
			}
			err = browser.OpenURL(consoleURL)
			if err != nil {
				logger.Warnf("failed opening a browser: %s", err)
			}
		}()
	}
	return nil
}

func (o *consoleOptions) beforeStartCleanUp(ce containerEngineInterface) error {
	clusterID, err := getClusterID()
	if err != nil {
		return fmt.Errorf("error getting cluster ID: %v", err)
	}
	containersToCleanUp := []string{
		fmt.Sprintf("monitoring-plugin-%s", clusterID),
		fmt.Sprintf("console-%s", clusterID),
	}

	logger.Infoln("Starting initial cleanup of containers")

	for _, c := range containersToCleanUp {
		exist, err := ce.containerIsExist(c)
		if err != nil {
			return fmt.Errorf("failed to check if container %s exists: %v", c, err)
		}
		if exist {
			err := ce.stopContainer(c)
			if err != nil {
				return fmt.Errorf("failed to stop container %s during the cleanup process: %v", c, err)
			}
		} else {
			logger.Infof("Container %s does not exist, no need to clean up", c)
		}
	}
	return nil
}

// cleanUp will first populate the containers needed to clean up, then call the blocking function
// o.terminateFunction, which will block until a system signal is received.
func (o *consoleOptions) cleanUp(ce containerEngineInterface) error {
	clusterID, err := getClusterID()
	if err != nil {
		return err
	}

	var containersToCleanUp []string

	// forcing order of removal as the order is not deterministic between container engines
	if o.needMonitorPlugin {
		logger.Debugln("adding monitoring plugin to containers for cleanup")
		containersToCleanUp = append(containersToCleanUp, fmt.Sprintf("monitoring-plugin-%s", clusterID))
	}
	containersToCleanUp = append(containersToCleanUp, fmt.Sprintf("console-%s", clusterID))

	// If for whatever reason the user did not call the proper function to create a console option
	// And the Cleanup method is called without a termination function defined
	if o.terminationFunction == nil {
		o.terminationFunction = &execActionOnTermStruct{}
	}

	err = o.terminationFunction.execActionOnTerminationFunction(func() error {
		for _, c := range containersToCleanUp {
			exist, err := ce.containerIsExist(c)
			if err != nil {
				return fmt.Errorf("failed to check container exist %s: %v", c, err)
			}
			if exist {
				err := ce.stopContainer(c)
				if err != nil {
					return fmt.Errorf("failed to stop container '%s' during the cleanup process: %v", c, err)
				}

				logger.Infoln(fmt.Sprintf("Container removed: %s", c))
			}

		}
		return nil
	})

	return err
}

// getClusterID returns the current cluster id in current kubeconfig
func getClusterID() (string, error) {
	currentClusterInfo, err := utils.DefaultClusterUtils.GetBackplaneClusterFromConfig()
	if err != nil {
		return "", err
	}
	return currentClusterInfo.ClusterID, nil
}

// getProxyURL returns the proxy url
func getProxyURL() (proxyURL *string, err error) {
	bpConfig, err := config.GetBackplaneConfiguration()
	if err != nil {
		return nil, err
	}

	return bpConfig.ProxyURL, nil
}

func getCurrentKubeconfig() (*rest.Config, error) {
	// Get the RESTconfig from the current kubeconfig context.
	cf := genericclioptions.NewConfigFlags(true)
	config, err := cf.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	return config, nil
}

// getConsoleImageFromCluster get the image from the console deployment
func getConsoleImageFromCluster(config *rest.Config) (string, error) {
	clientSet, err := createClientSet(config)
	if err != nil {
		return "", err
	}

	deploymentsClient := clientSet.AppsV1().Deployments(ConsoleNS)
	result, getErr := deploymentsClient.Get(context.TODO(), ConsoleDeployment, metav1.GetOptions{})
	if getErr != nil {
		return "", fmt.Errorf("failed to get console deployment: %v", getErr)
	}
	for _, container := range result.Spec.Template.Spec.Containers {
		if container.Name == "console" {
			return container.Image, nil
		}
	}
	return "", fmt.Errorf("could not find console container spec in console deployment")
}

// getMonitoringPluginImageFromCluster get the monitoring plugin image from deployment
func getMonitoringPluginImageFromCluster(config *rest.Config) (string, error) {
	clientSet, err := createClientSet(config)
	if err != nil {
		return "", err
	}

	deploymentsClient := clientSet.AppsV1().Deployments(MonitoringNS)
	result, getErr := deploymentsClient.Get(context.TODO(), MonitoringPluginDeployment, metav1.GetOptions{})
	if getErr != nil {
		return "", fmt.Errorf("failed to get monitoring-plugin deployment: %v", getErr)
	}
	for _, container := range result.Spec.Template.Spec.Containers {
		if container.Name == "monitoring-plugin" {
			return container.Image, nil
		}
	}
	return "", fmt.Errorf("could not find monitoring-plugin container spec in monitoring-plugin deployment")
}

// getConsolePluginFromCluster get the consoleplugin from the cluster
func getConsolePluginFromCluster(config *rest.Config) ([]string, error) {
	consoleInterface, err := consolev1typedclient.NewForConfig(config)

	if err != nil {
		return nil, err
	}
	consolePlugins, err := consoleInterface.ConsolePlugins().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var enabledPlugins []string

	for _, cp := range consolePlugins.Items {
		enabled, err := isConsolePluginEnabled(config, cp.Name)
		if err != nil {
			return nil, err
		}
		if enabled {
			enabledPlugins = append(enabledPlugins, fmt.Sprintf("%s=https://%s.%s.svc.cluster.local:%d%s",
				cp.Name, cp.Spec.Backend.Service.Name, cp.Spec.Backend.Service.Namespace, cp.Spec.Backend.Service.Port, cp.Spec.Backend.Service.BasePath))
		}
	}

	return enabledPlugins, nil
}

// getConsolePluginFrom411Cluster get the consoleplugin from the cluster with version lt 4.12
func getConsolePluginFrom411Cluster(config *rest.Config) ([]string, error) {
	consoleInterface, err := consolev1alpha1typedclient.NewForConfig(config)

	if err != nil {
		return nil, err
	}
	consolePlugins, err := consoleInterface.ConsolePlugins().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var enabledPlugins []string

	for _, cp := range consolePlugins.Items {
		enabled, err := isConsolePluginEnabled(config, cp.Name)
		if err != nil {
			return nil, err
		}
		if enabled {
			enabledPlugins = append(enabledPlugins, fmt.Sprintf("%s=https://%s.%s.svc.cluster.local:%d%s",
				cp.Name, cp.Spec.Service.Name, cp.Spec.Service.Namespace, cp.Spec.Service.Port, cp.Spec.Service.BasePath))
		}
	}

	return enabledPlugins, nil
}

// isConsolePluginEnabled checks if the consoleplugin object is enabled in console.operator
func isConsolePluginEnabled(config *rest.Config, consolePlugin string) (bool, error) {
	operatorInterface, err := operatorv1typedclient.NewForConfig(config)
	if err != nil {
		return false, err
	}

	consoleOperator, err := operatorInterface.Consoles().Get(context.TODO(), "cluster", metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	for _, plugin := range consoleOperator.Spec.Plugins {
		if plugin == consolePlugin {
			return true, nil
		}
	}

	return false, nil
}

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

// isRunningHigherOrEqualTo check if the cluster is running higher or equal to target version
func isRunningHigherOrEqualTo(targetVersionStr string) bool {
	var (
		clusterVersion *semver.Version
		targetVersion  *semver.Version
	)

	currentClusterInfo, err := utils.DefaultClusterUtils.GetBackplaneClusterFromConfig()
	if err != nil {
		return false
	}
	currentCluster, err := ocm.DefaultOCMInterface.GetClusterInfoByID(currentClusterInfo.ClusterID)

	if err != nil {
		return false
	}

	clusterVersionStr := currentCluster.OpenshiftVersion()
	if clusterVersionStr != "" {
		if clusterVersion, err = semver.NewVersion(clusterVersionStr); err != nil {
			return false
		}
		if targetVersion, err = semver.NewVersion(targetVersionStr); err != nil {
			return false
		}

		if clusterVersion.Equal(targetVersion) || clusterVersion.GreaterThan(targetVersion) {
			return true
		}
	}
	return false
}

// If a url is provided via consoleArgs.url, then the original url pointing to the homepage of the locally-running
// console will have its scheme and host inserted into consoleArgs.url.
// This is commonly used when trying to open a console url provided by PagerDuty or an end-user.
func replaceConsoleURL(containerURL string, userProvidedURL string) (string, error) {
	if len(userProvidedURL) == 0 {
		// no need to replace
		return containerURL, nil
	}

	o, err := url.Parse(containerURL)
	if err != nil {
		return "", err
	}

	// In PagerDuty, the entire URL is encoded such that it starts with https:/// (three forward slashes).
	// We need to replace it with two forward slashes so that it can be parsed as a valid URL.
	u, err := url.Parse(strings.Replace(userProvidedURL, "///", "//", 1))
	if err != nil {
		return "", err
	}

	u.Scheme = o.Scheme
	u.Host = o.Host

	return u.String(), nil
}

type postTerminateFunc func() error

// keep the program running in frontend and wait for ctrl-c
func (e *execActionOnTermStruct) execActionOnTerminationFunction(action postTerminateFunc) error {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigs
	fmt.Printf("System signal '%v' received, cleaning up containers and exiting...\n", sig)

	return action()
}

// ---- Container Engine Implementations ---- //

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
func (ce *podmanLinux) pullImage(imageName string) error {
	return generalPodmanPullImage(imageName)
}

// podman-pull for Mac
func (ce *podmanMac) pullImage(imageName string) error {
	return generalPodmanPullImage(imageName)
}

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
func (ce *dockerLinux) pullImage(imageName string) error {
	return generalDockerPullImage(imageName)
}

// docker-pull for Mac
func (ce *dockerMac) pullImage(imageName string) error {
	return generalDockerPullImage(imageName)
}

// the shared function for podman to run console container for both linux and macOS
func podmanRunConsoleContainer(containerName string, port string, consoleArgs []string, envVars []envVar) error {
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
			"--env", fmt.Sprintf("%s=%s", e.key, e.value),
		)
	}
	engRunArgs = append(engRunArgs, consoleArgs...)
	logger.WithField("Command", fmt.Sprintf("`%s %s`", PODMAN, strings.Join(engRunArgs, " "))).Infoln("Running container")

	runCmd := createCommand(PODMAN, engRunArgs...)
	runCmd.Stderr = os.Stderr
	runCmd.Stdout = nil

	return runCmd.Run()
}

func (ce *podmanMac) runConsoleContainer(containerName string, port string, consoleArgs []string, envVars []envVar) error {
	return podmanRunConsoleContainer(containerName, port, consoleArgs, envVars)
}

func (ce *podmanLinux) runConsoleContainer(containerName string, port string, consoleArgs []string, envVars []envVar) error {
	return podmanRunConsoleContainer(containerName, port, consoleArgs, envVars)
}

// the shared function for docker to run console container for both linux and macOS
func dockerRunConsoleContainer(containerName string, port string, consoleArgs []string, envVars []envVar) error {
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
			"--env", fmt.Sprintf("%s=%s", e.key, e.value),
		)
	}
	engRunArgs = append(engRunArgs, consoleArgs...)
	logger.WithField("Command", fmt.Sprintf("`%s %s`", DOCKER, strings.Join(engRunArgs, " "))).Infoln("Running container")

	runCmd := createCommand(DOCKER, engRunArgs...)
	runCmd.Stderr = os.Stderr
	runCmd.Stdout = nil

	return runCmd.Run()
}

func (ce *dockerMac) runConsoleContainer(containerName string, port string, consoleArgs []string, envVars []envVar) error {
	return dockerRunConsoleContainer(containerName, port, consoleArgs, envVars)
}

func (ce *dockerLinux) runConsoleContainer(containerName string, port string, consoleArgs []string, envVars []envVar) error {
	return dockerRunConsoleContainer(containerName, port, consoleArgs, envVars)
}

// the shared function for podman to run monitoring plugin for both linux and macOS
func podmanRunMonitorPlugin(containerName string, consoleContainerName string, nginxConfPath string, pluginArgs []string) error {
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
		"--mount", fmt.Sprintf("type=bind,source=%s,destination=/etc/nginx/nginx.conf,relabel=shared", nginxConfPath),
	}
	engRunArgs = append(engRunArgs, pluginArgs...)

	logger.WithField("Command", fmt.Sprintf("`%s %s`", PODMAN, strings.Join(engRunArgs, " "))).Infoln("Running container")
	runCmd := createCommand(PODMAN, engRunArgs...)
	runCmd.Stderr = os.Stderr
	runCmd.Stdout = nil

	return runCmd.Run()
}

func (ce *podmanMac) runMonitorPlugin(containerName string, consoleContainerName string, nginxConf string, pluginArgs []string) error {
	nginxConfPath := filepath.Join("/tmp/", nginxConf)
	return podmanRunMonitorPlugin(containerName, consoleContainerName, nginxConfPath, pluginArgs)
}

func (ce *podmanLinux) runMonitorPlugin(containerName string, consoleContainerName string, nginxConf string, pluginArgs []string) error {
	nginxConfPath := filepath.Join(ce.fileMountDir, nginxConf)
	return podmanRunMonitorPlugin(containerName, consoleContainerName, nginxConfPath, pluginArgs)
}

// the shared function for docker to run monitoring plugin for both linux and macOS
func dockerRunMonitorPlugin(containerName string, _ string, nginxConfPath string, pluginArgs []string) error {
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
		"--volume", fmt.Sprintf("%s:/etc/nginx/nginx.conf:z", nginxConfPath),
	}
	engRunArgs = append(engRunArgs, pluginArgs...)

	logger.WithField("Command", fmt.Sprintf("`%s %s`", DOCKER, strings.Join(engRunArgs, " "))).Infoln("Running container")
	runCmd := createCommand(DOCKER, engRunArgs...)
	runCmd.Stderr = os.Stderr
	runCmd.Stdout = nil

	return runCmd.Run()
}

func (ce *dockerLinux) runMonitorPlugin(containerName string, consoleContainerName string, nginxConf string, pluginArgs []string) error {
	configDirectory, err := config.GetConfigDirectory()
	if err != nil {
		return err
	}
	nginxConfPath := filepath.Join(configDirectory, nginxConf)
	return dockerRunMonitorPlugin(containerName, consoleContainerName, nginxConfPath, pluginArgs)
}

func (ce *dockerMac) runMonitorPlugin(containerName string, consoleContainerName string, nginxConf string, pluginArgs []string) error {
	configDirectory, err := config.GetConfigDirectory()
	if err != nil {
		return err
	}
	nginxConfPath := filepath.Join(configDirectory, nginxConf)
	return dockerRunMonitorPlugin(containerName, consoleContainerName, nginxConfPath, pluginArgs)
}

// put a file in place for container to mount
// filename should be name only, not a path
func (ce *podmanLinux) putFileToMount(filename string, content []byte) error {
	// ensure the directory exists
	err := os.MkdirAll(ce.fileMountDir, os.ModePerm)
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
	if err = os.Chmod(dstFileName, 0640); err != nil {
		logger.Debugf("change permission to 0640 for %s", dstFileName)
		return err
	}

	return nil
}

// put a file in place for container to mount
// filename should be name only, not a path
func (ce *podmanMac) putFileToMount(filename string, content []byte) error {
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

func (ce *dockerLinux) putFileToMount(filename string, content []byte) error {
	return dockerPutFileToMount(filename, content)
}

func (ce *dockerMac) putFileToMount(filename string, content []byte) error {
	return dockerPutFileToMount(filename, content)
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

// podman-stop for Linux
func (ce *podmanLinux) stopContainer(containerName string) error {
	return generalStopContainer(PODMAN, containerName)
}

// podman-stop for macOS
func (ce *podmanMac) stopContainer(containerName string) error {
	return generalStopContainer(PODMAN, containerName)
}

// docker-stop for Linux
func (ce *dockerLinux) stopContainer(containerName string) error {
	return generalStopContainer(DOCKER, containerName)
}

// docker-stop for macOS
func (ce *dockerMac) stopContainer(containerName string) error {
	return generalStopContainer(DOCKER, containerName)
}

// podman-exist for Linux
func (ce *podmanLinux) containerIsExist(containerName string) (bool, error) {
	return generalContainerIsExist(PODMAN, containerName)
}

// podman-exist for macOS
func (ce *podmanMac) containerIsExist(containerName string) (bool, error) {
	return generalContainerIsExist(PODMAN, containerName)
}

// docker-exist for Linux
func (ce *dockerLinux) containerIsExist(containerName string) (bool, error) {
	return generalContainerIsExist(DOCKER, containerName)
}

// docker-exist for macOS
func (ce *dockerMac) containerIsExist(containerName string) (bool, error) {
	return generalContainerIsExist(DOCKER, containerName)
}
