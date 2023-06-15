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
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	homedir "github.com/mitchellh/go-homedir"
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
	"github.com/openshift/backplane-cli/pkg/utils"
)

const (
	// DOCKER binary name of docker
	DOCKER = "docker"
	// PODMAN binary name of podman
	PODMAN = "podman"
)

var (
	consoleArgs struct {
		image           string
		port            string
		containerEngine string
		url             string
		openBrowser     bool
		enablePlugins   bool
	}
	validContainerEngines = []string{PODMAN, DOCKER}
	// For mocking
	createClientSet = func(c *rest.Config) (kubernetes.Interface, error) { return kubernetes.NewForConfig(c) }
	createCommand   = exec.Command

	// Pull Secret saving directory
	pullSecretConfigDirectory string
)

// Environment variable that indicates if open by browser is set as default
const EnvBrowserDefault = "BACKPLANE_DEFAULT_OPEN_BROWSER"

// ConsoleCmd represents the console command
var ConsoleCmd = &cobra.Command{
	Use:   "console",
	Short: "Launch Openshift console for the current cluster",
	Long: `console will start the Openshift console application locally for the currently logged in cluster.
Default behaviour is to run the same console image as the cluster.
Clusters below 4.8 will not display metrics, alerts, or dashboards. If you need to view metrics, alerts, or dashboards use the latest console image
with --image=quay.io/openshift/origin-console .
You can specify container engine with -c. If not specified, it will lookup the PATH in the order of podman and docker.
`,
	RunE:         runConsole,
	SilenceUsage: true,
}

func init() {
	flags := ConsoleCmd.Flags()
	flags.StringVar(
		&consoleArgs.image,
		"image",
		"",
		"Specify console image to use. Default: The same console image in the cluster",
	)
	flags.StringVar(
		&consoleArgs.port,
		"port",
		"",
		"Specify port to listen to. Default: A random available port",
	)
	flags.BoolVarP(
		&consoleArgs.openBrowser,
		"browser",
		"b",
		false,
		fmt.Sprintf("Open a browser after the console container starts. Can also be set via the environment variable '%s'", EnvBrowserDefault),
	)
	flags.StringVarP(
		&consoleArgs.containerEngine,
		"container-engine",
		"c",
		"",
		fmt.Sprintf("Specify container engine. -c %s.", strings.Join(validContainerEngines, "|")),
	)
	flags.BoolVarP(
		&consoleArgs.enablePlugins,
		"plugins",
		"",
		false,
		"Load enabled dynamic console plugins on the cluster. Default: false",
	)
	flags.StringVarP(
		&consoleArgs.url,
		"url",
		"u",
		"",
		"The full console url, e.g. from PagerDuty. The hostname will be replaced with that of the locally running console.",
	)
}

func checkContainerExists(containerName string, containerEngine string) (exists bool, err error) {
	existCheckArgs := []string{
		"container",
		"ps",
		"--filter",
		fmt.Sprintf("name=%s", containerName),
	}

	existCheckCmd, existCheckCmdOutput := createCommand(containerEngine, existCheckArgs...), new(strings.Builder)
	existCheckCmd.Stderr = os.Stderr
	existCheckCmd.Stdout = existCheckCmdOutput

	err = existCheckCmd.Run()
	if err != nil {
		return false, err
	}

	if strings.Contains(existCheckCmdOutput.String(), containerName) {
		return true, nil
	}

	return false, nil
}

func findConsoleAddress(containerName string, containerEngine string) (address string, err error) {
	addressCheckArgs := []string{
		"container",
		"inspect",
		"--format",
		"\"{{json .Config.Cmd}}\"",
		containerName,
	}
	addressCheckCmd, addressCheckOutput := createCommand(containerEngine, addressCheckArgs...), new(strings.Builder)
	addressCheckCmd.Stderr = os.Stderr
	addressCheckCmd.Stdout = addressCheckOutput

	err = addressCheckCmd.Run()
	if err != nil {
		return "", err
	}

	configurationCmdFragments := []string{}

	err = json.Unmarshal([]byte(strings.Trim(addressCheckOutput.String(), "\"\n ")), &configurationCmdFragments)
	if err != nil {
		return "", err
	}

	addressIndex := -1
	for idx, val := range configurationCmdFragments {
		if val == "-base-address" {
			addressIndex = idx + 1
			break
		}
	}
	if addressIndex == -1 || addressIndex >= len(configurationCmdFragments) {
		return "", fmt.Errorf("could not find address")
	}

	return configurationCmdFragments[addressIndex], nil
}

func checkAndFindContainerURL(containerName string, containerEngine string) (err error) {
	exists, err := checkContainerExists(containerName, containerEngine)
	if err != nil {
		return err
	}

	if exists {
		address, err := findConsoleAddress(containerName, containerEngine)
		if err != nil {
			return fmt.Errorf("console container is already running: %s", err)
		}

		return fmt.Errorf("console container is already running on: %s", address)
	}

	return nil
}

func runConsole(cmd *cobra.Command, argv []string) (err error) {
	// Check if env variable 'BACKPLANE_DEFAULT_OPEN_BROWSER' is set
	if env, ok := os.LookupEnv(EnvBrowserDefault); ok {
		// if set, try to parse it as a bool and pass it into consoleArgs.browser
		consoleArgs.openBrowser, err = strconv.ParseBool(env)
		if err != nil {
			return fmt.Errorf("unable to parse boolean value from environment variable %s", EnvBrowserDefault)
		}
	}
	// Pick a container engine
	// If user specify by -c, check if it exists.
	// Otherwise find an available engine in PATH
	containerEngine := ""
	if len(consoleArgs.containerEngine) > 0 {
		for _, ce := range validContainerEngines {
			if strings.EqualFold(consoleArgs.containerEngine, ce) {
				containerEngine = ce
			}
		}
		if len(containerEngine) == 0 {
			return fmt.Errorf("container engine can only be one of %s", strings.Join(validContainerEngines, "|"))
		}
		if _, err := exec.LookPath(containerEngine); err != nil {
			return fmt.Errorf("can't find %s in PATH", containerEngine)
		}
	} else {
		// Get the container engine via env vars
		engine, hasEngine := os.LookupEnv("CONTAINER_ENGINE")

		if hasEngine {
			containerEngine = engine
		} else {
			// Fetch container engine via path
			for _, ce := range validContainerEngines {
				if _, err := exec.LookPath(ce); err == nil {
					containerEngine = ce
					break
				}
			}
			if len(containerEngine) == 0 {
				return fmt.Errorf("can't find %s in PATH, please install one of the container engines", strings.Join(validContainerEngines, "|"))
			}
		}
	}
	logger.Infof("Using container engine %s\n", containerEngine)

	currentClusterInfo, err := utils.DefaultClusterUtils.GetBackplaneClusterFromConfig()
	if err != nil {
		return err
	}
	clusterId := currentClusterInfo.ClusterID

	consoleContainerName := fmt.Sprintf("console-%s", clusterId)

	err = checkAndFindContainerURL(consoleContainerName, containerEngine)
	if err != nil {
		return err
	}

	fmt.Printf("Starting console for cluster %s\n", clusterId)

	// Get the RESTconfig from the current kubeconfig context.
	cf := genericclioptions.NewConfigFlags(true)
	config, err := cf.ToRESTConfig()
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

	// Get image
	if len(consoleArgs.image) == 0 {
		logger.Debugln("Querying the cluster for console image")
		consoleArgs.image, err = getImageFromCluster(config)
		if err != nil {
			return err
		}
	}
	logger.Infof("Using image %s\n", consoleArgs.image)

	// Find a port to listen
	if len(consoleArgs.port) > 0 {
		_, err := strconv.Atoi(consoleArgs.port)
		if err != nil {
			return fmt.Errorf("port should be an integer")
		}
	} else {
		port, err := utils.GetFreePort()
		if err != nil {
			return fmt.Errorf("failed looking up a free port: %s", err)
		}
		consoleArgs.port = strconv.Itoa(port)
	}

	// Get ocm access token
	logger.Debugln("Finding ocm token")
	ocmToken, err := utils.DefaultOCMInterface.GetOCMAccessToken()
	if err != nil {
		return err
	}

	// Ensure we have authfile to pull image
	configDirectory, configFilename, err := fetchPullSecretIfNotExist()
	if err != nil {
		return err
	}

	// Engine-specific args.
	// Docker/Podman has different flags, we should treat them differently
	bridgeListen := fmt.Sprintf("http://0.0.0.0:%s", consoleArgs.port)
	engPullArgs := []string{"pull", "--quiet"}
	engRunArgs := []string{
		"run",
		"--rm",
		"--name", consoleContainerName,
		"-p", fmt.Sprintf("127.0.0.1:%s:%s", consoleArgs.port, consoleArgs.port),
	}

	if containerEngine == DOCKER {
		// in docker, --config should be made first
		engPullArgs = append(
			[]string{"--config", configDirectory},
			engPullArgs...,
		)
		engRunArgs = append(
			[]string{"--config", configDirectory},
			engRunArgs...,
		)
	}

	if containerEngine == PODMAN {
		engPullArgs = append(engPullArgs,
			"--authfile", configFilename,
		)
		engRunArgs = append(engRunArgs,
			"--authfile", configFilename,
		)
	}

	// Set proxy URL to the container
	proxyUrl, err := getProxyUrl()
	if err != nil {
		return err
	}

	if proxyUrl != "" {
		engRunArgs = append(engRunArgs,
			"--env", fmt.Sprintf("HTTPS_PROXY=%s", proxyUrl),
		)
	}

	// For docker on linux, we need to use host network,
	// otherwise it won't go through the sshuttle.
	if runtime.GOOS == "linux" && containerEngine == DOCKER {
		logger.Debugln("using host network for docker running on linux")
		engRunArgs = append(engRunArgs,
			"--network", "host",
		)
		// listen to loopback only for security
		bridgeListen = fmt.Sprintf("http://127.0.0.1:%s", consoleArgs.port)
	}

	// Pull the console image
	pullArgs := append(engPullArgs,
		consoleArgs.image,
	)

	c, err := utils.DefaultOCMInterface.GetClusterInfoByID(clusterId)
	if err != nil {
		return err
	}
	p, ok := c.GetProduct()
	if !ok {
		return fmt.Errorf("Could not get product information")
	}

	logger.WithField("Command", fmt.Sprintf("`%s %s`", containerEngine, strings.Join(pullArgs, " "))).Infoln("Pulling image")
	pullCmd := createCommand(containerEngine, pullArgs...)
	pullCmd.Stderr = os.Stderr
	pullCmd.Stdout = nil
	err = pullCmd.Run()
	if err != nil {
		return err
	}

	branding := "dedicated"
	documentationUrl := "https://docs.openshift.com/dedicated/4/"
	if p.ID() == "rosa" {
		branding = "ocp"
		documentationUrl = "https://docs.openshift.com/rosa/"
	}

	// Run the console container
	containerArgs := append(engRunArgs,
		consoleArgs.image,
		"/opt/bridge/bin/bridge",
		"--public-dir=/opt/bridge/static",
		"-base-address", fmt.Sprintf("http://127.0.0.1:%s", consoleArgs.port),
		"-branding", branding,
		"-documentation-base-url", documentationUrl,
		"-user-settings-location", "localstorage",
		"-user-auth", "disabled",
		"-k8s-mode", "off-cluster",
		"-k8s-auth", "bearer-token",
		"-k8s-mode-off-cluster-endpoint", apiURL,
		"-k8s-mode-off-cluster-alertmanager", alertmanagerURL,
		"-k8s-mode-off-cluster-thanos", thanosURL,
		"-k8s-auth-bearer-token", *ocmToken,
		"-listen", bridgeListen,
	)

	if consoleArgs.enablePlugins {
		consolePlugins, err := loadConsolePlugins(config)
		if err != nil {
			return err
		}
		if len(consolePlugins) > 0 {
			containerArgs = append(containerArgs, "-plugins", consolePlugins)
		}
	}

	// Store the locally running console URL or splice it into a url provided in consoleArgs.url
	consoleUrl, err := replaceConsoleUrl(fmt.Sprintf("http://127.0.0.1:%s", consoleArgs.port))
	if err != nil {
		return fmt.Errorf("failed to replace url: %v", err)
	}

	fmt.Printf("== Console is available at %s ==\n\n", consoleUrl)
	logger.WithField("Command", fmt.Sprintf("`%s %s`", containerEngine, strings.Join(containerArgs, " "))).Infoln("Running container")

	if consoleArgs.openBrowser {
		go func() {
			err := wait.PollImmediate(time.Second, 5*time.Second, func() (bool, error) {
				return utils.CheckHealth(fmt.Sprintf("%s/health", consoleUrl)), nil
			})
			if err != nil {
				logger.Warnf("failed waiting for container to become ready: %s", err)
				return
			}
			err = browser.OpenURL(consoleUrl)
			if err != nil {
				logger.Warnf("failed opening a browser: %s", err)
			}
		}()
	}

	containerCmd := createCommand(containerEngine, containerArgs...)
	if logger.GetLevel() >= logger.InfoLevel {
		containerCmd.Stderr = os.Stderr
		containerCmd.Stdout = os.Stdin
	} else {
		containerCmd.Stderr = nil
		containerCmd.Stdout = nil
	}
	err = containerCmd.Run()

	return err
}

// If a url is provided via consoleArgs.url, then the original url pointing to the homepage of the locally-running
// console will have its scheme and host inserted into consoleArgs.url.
// This is commonly used when trying to open a console url provided by PagerDuty or an end-user.
func replaceConsoleUrl(original string) (string, error) {
	if consoleArgs.url != "" {
		o, err := url.Parse(original)
		if err != nil {
			return "", err
		}

		// In PagerDuty, the entire URL is encoded such that it starts with https:/// (three forward slashes).
		// We need to replace it with two forward slashes so that it can be parsed as a valid URL.
		u, err := url.Parse(strings.Replace(consoleArgs.url, "///", "//", 1))
		if err != nil {
			return "", err
		}

		u.Scheme = o.Scheme
		u.Host = o.Host

		return u.String(), nil
	}

	return original, nil
}

// Get the proxy url
func getProxyUrl() (proxyUrl string, err error) {
	bpConfig, err := config.GetBackplaneConfiguration()

	if err != nil {
		return "", err
	}

	proxyUrl = bpConfig.ProxyURL

	return proxyUrl, nil
}

// getImageFromCluster get the image from the console deployment
func getImageFromCluster(config *rest.Config) (string, error) {
	clientSet, err := createClientSet(config)
	if err != nil {
		return "", err
	}

	deploymentsClient := clientSet.AppsV1().Deployments("openshift-console")
	result, getErr := deploymentsClient.Get(context.TODO(), "console", metav1.GetOptions{})
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

	response, err := utils.DefaultOCMInterface.GetPullSecret()
	if err != nil {
		return "", "", fmt.Errorf("failed to get pull secret from ocm: %v", err)
	}
	err = os.WriteFile(configFilename, []byte(response), 0600)
	if err != nil {
		return "", "", fmt.Errorf("failed to write authfile for pull secret: %v", err)
	}

	return configDirectory, configFilename, nil
}

// getConsolePluginFromCluster get the consoleplugin from the cluster
func getConsolePluginFromCluster(config *rest.Config) (string, error) {
	consoleInterface, err := consolev1typedclient.NewForConfig(config)

	if err != nil {
		return "", err
	}
	consolePlugins, err := consoleInterface.ConsolePlugins().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	var enabledPlugins []string

	for _, cp := range consolePlugins.Items {
		enabled, err := isConsolePluginEnabled(config, cp.Name)
		if err != nil {
			return "", err
		}
		if enabled {
			enabledPlugins = append(enabledPlugins, fmt.Sprintf("%s=https://%s.%s.svc.cluster.local:%d%s",
				cp.Name, cp.Spec.Backend.Service.Name, cp.Spec.Backend.Service.Namespace, cp.Spec.Backend.Service.Port, cp.Spec.Backend.Service.BasePath))
		}
	}

	return strings.Join(enabledPlugins, ","), nil
}

// getConsolePluginFrom411Cluster get the consoleplugin from the cluster with version lt 4.12
func getConsolePluginFrom411Cluster(config *rest.Config) (string, error) {
	consoleInterface, err := consolev1alpha1typedclient.NewForConfig(config)

	if err != nil {
		return "", err
	}
	consolePlugins, err := consoleInterface.ConsolePlugins().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	var enabledPlugins []string

	for _, cp := range consolePlugins.Items {
		enabled, err := isConsolePluginEnabled(config, cp.Name)
		if err != nil {
			return "", err
		}
		if enabled {
			enabledPlugins = append(enabledPlugins, fmt.Sprintf("%s=https://%s.%s.svc.cluster.local:%d%s",
				cp.Name, cp.Spec.Service.Name, cp.Spec.Service.Namespace, cp.Spec.Service.Port, cp.Spec.Service.BasePath))
		}
	}

	return strings.Join(enabledPlugins, ","), nil
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

// isRunningHigherThan411 checks the running cluster is higher than 411
func isRunningHigherThan411() bool {
	currentClusterInfo, err := utils.DefaultClusterUtils.GetBackplaneClusterFromConfig()
	if err != nil {
		return false
	}
	currentCluster, err := utils.DefaultOCMInterface.GetClusterInfoByID(currentClusterInfo.ClusterID)

	if err != nil {
		return false
	}
	clusterVersion := currentCluster.OpenshiftVersion()
	if clusterVersion != "" {
		version, err := semver.NewVersion(clusterVersion)
		if err != nil {
			return false
		}
		if version.Minor() >= 12 {
			return true
		}
	}
	return false
}

// loadConsolePlugins load the enabled console plugins from the cluster when the flag --plugins set
func loadConsolePlugins(config *rest.Config) (string, error) {
	var consolePlugins string
	var err error

	if isRunningHigherThan411() {
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

	return consolePlugins, nil
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
		if err != nil {
			return "", fmt.Errorf("can't modify config directory. Error: %s", err.Error())
		}
	}

	return pullSecretConfigDirectory, nil
}
