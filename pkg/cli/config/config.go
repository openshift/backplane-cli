package config

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	logger "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/openshift/backplane-cli/pkg/info"
	"github.com/openshift/backplane-cli/pkg/ocm"
)

type JiraTransitionsNamesForAccessRequests struct {
	OnCreation string `json:"on-creation"`
	OnApproval string `json:"on-approval"`
	OnError    string `json:"on-error"`
}

type AccessRequestsJiraConfiguration struct {
	DefaultProject            string                                           `json:"default-project"`
	DefaultIssueType          string                                           `json:"default-issue-type"`
	ProdProject               string                                           `json:"prod-project"`
	ProdIssueType             string                                           `json:"prod-issue-type"`
	ProjectToTransitionsNames map[string]JiraTransitionsNamesForAccessRequests `json:"project-to-transitions-names"`
}

// Please update the validateConfig function if there is any required config key added
type BackplaneConfiguration struct {
	URL                         string                          `json:"url"`
	ProxyURL                    *string                         `json:"proxy-url"`
	SessionDirectory            string                          `json:"session-dir"`
	AssumeInitialArn            string                          `json:"assume-initial-arn"`
	ProdEnvName                 string                          `json:"prod-env-name"`
	PagerDutyAPIKey             string                          `json:"pd-key"`
	JiraBaseURL                 string                          `json:"jira-base-url"`
	JiraToken                   string                          `json:"jira-token"`
	JiraConfigForAccessRequests AccessRequestsJiraConfiguration `json:"jira-config-for-access-requests"`
	VPNCheckEndpoint            string                          `json:"vpn-check-endpoint"`
	ProxyCheckEndpoint          string                          `json:"proxy-check-endpoint"`
	DisplayClusterInfo          bool                            `json:"display-cluster-info"`
	Govcloud                    bool                            `json:"govcloud"`
}

const (
	prodEnvNameKey                      = "prod-env-name"
	jiraBaseURLKey                      = "jira-base-url"
	JiraTokenViperKey                   = "jira-token"
	JiraConfigForAccessRequestsKey      = "jira-config-for-access-requests"
	prodEnvNameDefaultValue             = "production"
	JiraBaseURLDefaultValue             = "https://issues.redhat.com"
	proxyTestTimeout                    = 10 * time.Second
	GovcloudDefaultValue           bool = false
	GovcloudDefaultValueKey             = "govcloud"
)

var JiraConfigForAccessRequestsDefaultValue = AccessRequestsJiraConfiguration{
	DefaultProject:   "SDAINT",
	DefaultIssueType: "Story",
	ProdProject:      "OHSS",
	ProdIssueType:    "Incident",
	ProjectToTransitionsNames: map[string]JiraTransitionsNamesForAccessRequests{
		"SDAINT": {
			OnCreation: "In Progress",
			OnApproval: "In Progress",
			OnError:    "Closed",
		},
		"OHSS": {
			OnCreation: "Pending Customer",
			OnApproval: "New",
			OnError:    "Cancelled",
		},
	},
}

// GetConfigFilePath returns the Backplane CLI configuration filepath
func GetConfigFilePath() (string, error) {
	// Check if user has explicitly defined backplane config path
	path, found := os.LookupEnv(info.BackplaneConfigPathEnvName)
	if found {
		return path, nil
	}

	UserHomeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	configFilePath := filepath.Join(UserHomeDir, info.BackplaneConfigDefaultFilePath, info.BackplaneConfigDefaultFileName)

	return configFilePath, nil
}

// GetBackplaneConfiguration parses and returns the given backplane configuration
func GetBackplaneConfiguration() (bpConfig BackplaneConfiguration, err error) {
	viper.SetDefault(prodEnvNameKey, prodEnvNameDefaultValue)
	viper.SetDefault(jiraBaseURLKey, JiraBaseURLDefaultValue)
	viper.SetDefault(JiraConfigForAccessRequestsKey, JiraConfigForAccessRequestsDefaultValue)
	viper.SetDefault(GovcloudDefaultValueKey, GovcloudDefaultValue)
	filePath, err := GetConfigFilePath()
	if err != nil {
		return bpConfig, err
	}

	viper.AutomaticEnv()

	// Check if the config file exists
	if _, err = os.Stat(filePath); err == nil {
		// Load config file
		viper.SetConfigFile(filePath)
		viper.SetConfigType("json")
		logger.Debugf("Reading config file %s", filePath)

		if err := viper.ReadInConfig(); err != nil {
			return bpConfig, err
		}
	}

	if err = validateConfig(); err != nil {
		logger.Warn(err)
	}

	bpConfig.Govcloud = viper.GetBool("govcloud")

	if !(bpConfig.Govcloud) {
		// Check if user has explicitly defined proxy; it has higher precedence over the config file
		err = viper.BindEnv("proxy-url", info.BackplaneProxyEnvName)
		if err != nil {
			return bpConfig, err
		}
	} else {
		logger.Debug("This is govcloud, no proxy to use")
	}

	// Warn user if url defined in the config file
	if viper.GetString("url") != "" {
		logger.Warn("Manual URL configuration is deprecated, please remove URL key from Backplane configuration")
	}

	// Warn if user has explicitly defined backplane URL via env
	url, ok := getBackplaneEnv(info.BackplaneURLEnvName)
	if ok {
		logger.Warn(fmt.Sprintf("Manual URL configuration is deprecated, please unset the environment %s", info.BackplaneURLEnvName))
		bpConfig.URL = url
	} else {
		// Fetch backplane URL from ocm env
		if bpConfig.URL, err = bpConfig.GetBackplaneURL(); err != nil {
			return bpConfig, err
		}
	}

	// proxyURL is required
	proxyInConfigFile := viper.GetStringSlice("proxy-url")
	proxyURL := bpConfig.getFirstWorkingProxyURL(proxyInConfigFile)
	if proxyURL != "" {
		bpConfig.ProxyURL = &proxyURL
	}

	if (bpConfig.Govcloud) {
		str := ""
		bpConfig.ProxyURL = &str
	}

	bpConfig.SessionDirectory = viper.GetString("session-dir")
	bpConfig.AssumeInitialArn = viper.GetString("assume-initial-arn")
	bpConfig.DisplayClusterInfo = viper.GetBool("display-cluster-info")

	// pagerDuty token is optional. Don't even check for FedRAMP
	if !(bpConfig.Govcloud) {
		pagerDutyAPIKey := viper.GetString("pd-key")
		if pagerDutyAPIKey != "" {
			bpConfig.PagerDutyAPIKey = pagerDutyAPIKey
		} else {
			logger.Info("No PagerDuty API Key configuration available. This will result in failure of `ocm-backplane login --pd <incident-id>` command.")
		}
	} else {
		logger.Debug("No PagerDuty API Key to use in govcloud")
	}

	// OCM prod env name is optional as there is a default value
	bpConfig.ProdEnvName = viper.GetString(prodEnvNameKey)

	// JIRA base URL is optional as there is a default value
	bpConfig.JiraBaseURL = viper.GetString(jiraBaseURLKey)
	if bpConfig.JiraBaseURL != "" {
		parsedURL, parseErr := url.ParseRequestURI(bpConfig.JiraBaseURL)
		if parseErr != nil || parsedURL.Scheme != "https" {
			logger.Warnf("Invalid JiraBaseURL '%s': not a valid HTTPS URL. Proceeding with potentially insecure or invalid URL.", bpConfig.JiraBaseURL)
		}
	}

	// JIRA token is optional
	bpConfig.JiraToken = viper.GetString(JiraTokenViperKey)

	// JIRA config for access requests is optional as there is a default value
	err = viper.UnmarshalKey(JiraConfigForAccessRequestsKey, &bpConfig.JiraConfigForAccessRequests)

	if err != nil {
		logger.Warnf("failed to unmarshal '%s' entry as json in '%s' config file: %v", JiraConfigForAccessRequestsKey, filePath, err)
	} else {
		for _, project := range []string{bpConfig.JiraConfigForAccessRequests.DefaultProject, bpConfig.JiraConfigForAccessRequests.ProdProject} {
			if _, isKnownProject := bpConfig.JiraConfigForAccessRequests.ProjectToTransitionsNames[project]; !isKnownProject {
				logger.Warnf("content unmarshalled from '%s' in '%s' config file is inconsistent: no transitions defined for project '%s'", JiraConfigForAccessRequestsKey, filePath, project)
			}
		}
	}

	// Load VPN and Proxy check endpoints from the local backplane configuration file
	// Don't even check for FedRAMP
	if !(bpConfig.Govcloud) {
		bpConfig.VPNCheckEndpoint = viper.GetString("vpn-check-endpoint")
		bpConfig.ProxyCheckEndpoint = viper.GetString("proxy-check-endpoint")
	} else {
		logger.Debug("This is govcloud, no VPN and Proxy check endpoints to use")
		bpConfig.VPNCheckEndpoint = ""
		bpConfig.ProxyCheckEndpoint = ""
	}
	return bpConfig, nil
}

var testProxy = func(ctx context.Context, testURL string, proxyURL url.URL) error {
	// Try call the test URL via the proxy
	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(&proxyURL)},
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", testURL, nil)
	resp, err := client.Do(req)

	// Check the result
	if err != nil {
		return fmt.Errorf("proxy returned an error %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected response code 200 but got %d", resp.StatusCode)
	}

	return nil
}

func (config *BackplaneConfiguration) getFirstWorkingProxyURL(s []string) string {
	if len(s) == 0 {
		logger.Debug("No proxy to use")
		return ""
	}

	// If we only have one proxy, there is no need to waste time on tests, just use that one
	if len(s) == 1 {
		logger.Debug("Only one proxy to choose from, automatically using it")
		return s[0]
	}

	// Context to time out or cancel all tests once we are done
	ctx, cancel := context.WithTimeout(context.Background(), proxyTestTimeout)
	var wg sync.WaitGroup
	ch := make(chan *url.URL)

	bpURL := config.URL + "/healthz"

	failures := 0
	for _, p := range s {
		// Parse the proxy URL
		proxyURL, err := url.ParseRequestURI(p)
		if err != nil {
			logger.Debugf("proxy-url: '%v' could not be parsed.", p)
			failures++
			continue
		}

		wg.Add(1)
		go func(proxyURL url.URL) {
			defer wg.Done()

			// Do the proxy test
			proxyErr := testProxy(ctx, bpURL, proxyURL)
			if proxyErr != nil {
				logger.Infof("Discarding proxy %s due to error: %s", proxyURL.String(), proxyErr)
				ch <- nil
				return
			}

			// This test succeeded, send to the main thread
			ch <- &proxyURL
		}(*proxyURL)
	}

	// Default to the first
	chosenURL := s[0]

	// Loop until all tests have failed or we get a single success
loop:
	for failures < len(s) {
		select {
		case proxyURL := <-ch: // A proxy returned a result
			// nil means the test failed
			if proxyURL == nil {
				failures++
				continue
			}

			// This proxy passed
			chosenURL = proxyURL.String()
			logger.Infof("proxy that responded first was %s", chosenURL)

			break loop

		case <-ctx.Done(): // We timed out waiting for a proxy to pass
			logger.Warnf("falling back to first proxy-url after all proxies timed out: %s", s[0])

			break loop
		}
	}

	// Cancel any remaining requests
	cancel()

	// Ignore any other valid proxies, until the channel is closed
	go func() {
		for lateProxy := range ch {
			if lateProxy != nil {
				logger.Infof("proxy %s responded too late", lateProxy)
			}
		}
	}()

	// Wait for goroutines to end, then close the channel
	wg.Wait()
	close(ch)

	return chosenURL
}

func validateConfig() error {

	// No Proxy used in FedRAMP
	if !(viper.GetBool("govcloud")) {
		// Validate the proxy url
		if viper.GetStringSlice("proxy-url") == nil && os.Getenv(info.BackplaneProxyEnvName) == "" {
			return fmt.Errorf("proxy-url must be set explicitly in either config file or via the environment HTTPS_PROXY")
		}
	}

	return nil
}

// GetConfigDirectory returns the backplane config path
func GetConfigDirectory() (string, error) {
	bpConfigFilePath, err := GetConfigFilePath()
	if err != nil {
		return "", err
	}
	configDirectory := filepath.Dir(bpConfigFilePath)

	return configDirectory, nil
}

// GetBackplaneURL returns API URL
func (config *BackplaneConfiguration) GetBackplaneURL() (string, error) {

	ocmEnv, err := ocm.DefaultOCMInterface.GetOCMEnvironment()
	if err != nil {
		return "", err
	}
	url, ok := ocmEnv.GetBackplaneURL()
	if !ok {
		return "", fmt.Errorf("the requested API endpoint is not available for the OCM environment: %v", ocmEnv.Name())
	}
	logger.Infof("Backplane URL retrieved via OCM environment: %s", url)
	return url, nil
}

// getBackplaneEnv retrieves the value of the environment variable named by the key
func getBackplaneEnv(key string) (string, bool) {
	val, ok := os.LookupEnv(key)
	if ok {
		logger.Infof("Backplane key %s set via env vars: %s", key, val)
		return val, ok
	}
	return "", false
}

// CheckAPIConnection validate API connection via configured proxy and VPN
func (config BackplaneConfiguration) CheckAPIConnection() error {

	// make test api connection
	connectionOk, err := config.testHTTPRequestToBackplaneAPI()

	if !connectionOk {
		return err
	}

	return nil
}

// testHTTPRequestToBackplaneAPI returns status of the API connection
func (config BackplaneConfiguration) testHTTPRequestToBackplaneAPI() (bool, error) {
	client := http.Client{
		Timeout: 5 * time.Second,
	}

	if config.ProxyURL != nil {
		proxyURL, err := url.Parse(*config.ProxyURL)
		if err != nil {
			return false, err
		}
		http.DefaultTransport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	}

	req, err := http.NewRequest("HEAD", config.URL, nil)
	if err != nil {
		return false, err
	}
	_, err = client.Do(req)
	if err != nil {
		return false, err
	}

	return true, nil
}