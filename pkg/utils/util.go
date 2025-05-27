package utils

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"reflect"
	"strings"

	netUrl "net/url"

	logger "github.com/sirupsen/logrus"

	BackplaneApi "github.com/openshift/backplane-api/pkg/client"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/openshift/backplane-cli/internal/github"
	"github.com/openshift/backplane-cli/pkg/info"
)

const (
	ClustersPageSize             = 50
	BackplaneAPIURLRegexp string = `(?mi)^https:\/\/api\.(.*)backplane\.(.*)`
	ClusterIDRegexp       string = "/?backplane/cluster/([a-zA-Z0-9]+)/?"
)

var (
	defaultKubeConfig = api.Config{
		Kind:        "Config",
		APIVersion:  "v1",
		Preferences: api.Preferences{},
		Clusters: map[string]*api.Cluster{
			"dummy_cluster": {
				Server: "https://api-backplane.apps.something.com/backplane/cluster/configcluster",
			},
		},
		Contexts: map[string]*api.Context{
			"default/test123/anonymous": {
				Cluster:   "dummy_cluster",
				Namespace: "default",
			},
		},
		CurrentContext: "default/test123/anonymous",
	}
	defaultKubeConfigFileName = "config"
)

// GetFreePort asks the OS for an available port to listen to.
// https://github.com/phayes/freeport/blob/master/freeport.go
func GetFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// CheckHealth check if the given url returns http status 200
// return false if it not 200 or encounter any error.
func CheckHealth(url string) bool {
	// Parse the given URL and check for ambiguities
	parsedURL, err := netUrl.Parse(url)
	if err != nil {
		return false //just return false for any error
	}

	resp, err := http.Get(parsedURL.String())
	if err != nil {
		return false //just return false for any error
	}
	return resp.StatusCode == http.StatusOK
}

func ReadKubeconfigRaw() (api.Config, error) {
	return genericclioptions.NewConfigFlags(true).ToRawKubeConfigLoader().RawConfig()
}

// MatchBaseDomain returns true if the given longHostname matches the baseDomain.
func MatchBaseDomain(longHostname, baseDomain string) bool {
	if len(baseDomain) == 0 {
		return true
	}
	hostnameSegs := strings.Split(longHostname, ".")
	baseSegs := strings.Split(baseDomain, ".")
	if len(hostnameSegs) < len(baseSegs) {
		return false
	}
	cmpSegs := hostnameSegs[len(hostnameSegs)-len(baseSegs):]

	return reflect.DeepEqual(cmpSegs, baseSegs)
}

func TryParseBackplaneAPIError(rsp *http.Response) (*BackplaneApi.Error, error) {
	if rsp == nil {
		return nil, fmt.Errorf("parse err provided nil http response")
	}
	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() {
		_ = rsp.Body.Close()
	}()
	if err != nil {
		return nil, err
	} else {
		var dest BackplaneApi.Error
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			bodyStr := strings.ReplaceAll(string(bodyBytes), "\n", " ")
			maxLen := 200
			if len(bodyStr) > maxLen {
				bodyStr = bodyStr[:maxLen] + "..."
			}
			// Return an error that includes the status, code, a truncated body, and the original unmarshal error.
			return nil, fmt.Errorf("status:'%s', code:'%d'; failed to unmarshal JSON response from backplane (body starts with: '%s'). Original error: %w", rsp.Status, rsp.StatusCode, bodyStr, err)
		}
		return &dest, nil
	}
}

func TryRenderErrorRaw(rsp *http.Response) error {
	data, err := TryParseBackplaneAPIError(rsp)
	if err != nil {
		return err
	}
	return RenderJSONBytes(data)
}

func GetFormattedError(rsp *http.Response) error {
	data, err := TryParseBackplaneAPIError(rsp)
	if err != nil {
		return err
	}
	if data.Message != nil && data.StatusCode != nil {
		return fmt.Errorf("error from backplane: \n Status Code: %d\n Message: %s", *data.StatusCode, *data.Message)
	} else {
		return fmt.Errorf("error from backplane: \n Status Code: %d\n Message: %s", rsp.StatusCode, rsp.Status)
	}
}

func TryPrintAPIError(rsp *http.Response, rawFlag bool) error {
	if rawFlag {
		if err := TryRenderErrorRaw(rsp); err != nil {
			return fmt.Errorf("unable to parse error from backplane: \n Status Code: %d", rsp.StatusCode)
		} else {
			return nil
		}
	} else {
		return GetFormattedError(rsp)
	}
}

func ParseParamsFlag(paramsFlag []string) (map[string]string, error) {
	var result = map[string]string{}
	for _, s := range paramsFlag {
		parts := strings.SplitN(s, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if key == "" {
				return nil, fmt.Errorf("error parsing params flag: key cannot be empty in '%s'", s)
			}
			result[key] = value
		} else {
			// This means there was no equals sign in 's'
			return nil, fmt.Errorf("error parsing params flag: missing '=' in '%s'", s)
		}
	}
	return result, nil
}

func CreateTempKubeConfig(kubeConfig *api.Config) error {

	f, err := os.CreateTemp("", defaultKubeConfigFileName)
	if err != nil {
		return err
	}
	// set default kube config if values are empty
	if kubeConfig == nil {
		kubeConfig = &defaultKubeConfig
	}
	err = clientcmd.WriteToFile(*kubeConfig, f.Name())

	if err != nil {
		return err
	}

	err = f.Close()
	if err != nil {
		return err
	}

	// set kube config env with temp kube config file
	os.Setenv("KUBECONFIG", f.Name())
	return nil

}

// GetDefaultKubeConfig return default kube config
func GetDefaultKubeConfig() api.Config {
	return defaultKubeConfig
}

// ModifyTempKubeConfigFileName update default temp kube config file name
func ModifyTempKubeConfigFileName(fileName string) error {
	defaultKubeConfigFileName = fileName

	return nil
}

func RemoveTempKubeConfig() {
	path, found := os.LookupEnv("KUBECONFIG")
	if found {
		os.Remove(path)
	}
}

// CheckBackplaneVersion checks the backplane version and aims to only
// report any errors encountered in the process in order to
// avoid calling functions act as usual
func CheckBackplaneVersion(cmd *cobra.Command) {
	if cmd == nil {
		logger.Debugln("Command object is nil")
		return
	}

	ctx := cmd.Context()
	if ctx == nil {
		logger.Debugln("Context object is nil")
		return
	}

	git := github.NewClient()
	if err := git.CheckConnection(); err != nil {
		logger.WithField("Connection error", err).Warn("Could not connect to GitHub")
		return
	}

	// Get the latest version from the GitHub API
	latestVersionTag, err := git.GetLatestVersion(ctx)
	if err != nil {
		logger.WithField("Fetch error", err).Warn("Could not fetch latest version from GitHub")
		return
	}
	// GitHub API keeps the v prefix in front which causes mismatch with info.Version
	latestVersion := strings.TrimLeft(latestVersionTag.TagName, "v")

	currentVersion := info.DefaultInfoService.GetVersion()
	// Check if the local version is already up-to-date
	if latestVersion == currentVersion {
		logger.WithField("Current version", currentVersion).Info("Already up-to-date")
		return
	}

	logger.WithField("Current version", currentVersion).WithField("Latest version", latestVersion).Warn("Your Backplane CLI is not up to date. Please run the command 'ocm backplane upgrade' to upgrade to the latest version")
}

// CheckValidPrompt checks that the stdin and stderr are valid for prompt
// and are not provided by a pipe or file
func CheckValidPrompt() bool {
	stdin, _ := os.Stdin.Stat()
	stdout, _ := os.Stderr.Stat()
	return (stdin.Mode()&os.ModeCharDevice) != 0 && (stdout.Mode()&os.ModeCharDevice) != 0
}

// AskQuestionFromPrompt will first check if stdIn/Err are valid for promting, if not the it will just return empty string
// otherwise if will display the question to stderr and read answer as returned string
func AskQuestionFromPrompt(question string) string {
	if CheckValidPrompt() {
		// Create a new scanner to read from stdin
		scanner := bufio.NewScanner(os.Stdin)
		os.Stderr.WriteString(question)
		// Read the entire line (until the user presses Enter)
		if scanner.Scan() {
			return scanner.Text()
		}
	}
	return ""
}

// AppendUniqNoneEmptyString will append a string to a slice if that string is not empty and is not already part of the slice
func AppendUniqNoneEmptyString(slice []string, element string) []string {
	if element == "" {
		return slice
	}
	for _, existing := range slice {
		if existing == element {
			return slice // Element already exists, no need to add
		}
	}
	return append(slice, element) // Append the element
}