package config

import (
	"context"
	"fmt"
	"net/http"
	"time"

	BackplaneApi "github.com/openshift/backplane-api/pkg/client"
	logger "github.com/sirupsen/logrus"
)

//go:generate mockgen -destination=mocks/mock_config_api_client.go -package=mocks github.com/openshift/backplane-cli/pkg/cli/config ConfigAPIClient

// ConfigAPIClient handles fetching configuration from backplane-api
type ConfigAPIClient interface {
	FetchRemoteConfig(client BackplaneApi.ClientInterface) (*RemoteConfig, error)
}

// DefaultConfigAPIClient implements ConfigAPIClient using the real API
type DefaultConfigAPIClient struct{}

// DefaultAPIClient is the default instance used for API calls
var DefaultAPIClient ConfigAPIClient = &DefaultConfigAPIClient{}

// RemoteConfig represents the config values fetched from /config endpoint
type RemoteConfig struct {
	JiraBaseURL                 *string
	AssumeInitialArn            *string
	ProdEnvName                 *string
	JiraConfigForAccessRequests *AccessRequestsJiraConfiguration
}

// FetchRemoteConfig calls the /config endpoint and parses the response
func (c *DefaultConfigAPIClient) FetchRemoteConfig(client BackplaneApi.ClientInterface) (*RemoteConfig, error) {
	logger.Debugf("Fetching remote configuration from /backplane/config")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.GetConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to call /config endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code from /config endpoint: %d", resp.StatusCode)
	}

	configResp, err := BackplaneApi.ParseGetConfigResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse /config response: %w", err)
	}

	if configResp.JSON200 == nil {
		return nil, fmt.Errorf("empty response from /config endpoint")
	}

	return mapAPIResponseToRemoteConfig(configResp.JSON200), nil
}

// mapAPIResponseToRemoteConfig converts API response to internal RemoteConfig
func mapAPIResponseToRemoteConfig(apiResp *BackplaneApi.ClientConfig) *RemoteConfig {
	remote := &RemoteConfig{}

	// Map simple string fields
	if apiResp.JiraBaseUrl != nil {
		remote.JiraBaseURL = apiResp.JiraBaseUrl
	}

	if apiResp.AssumeInitialArn != nil {
		remote.AssumeInitialArn = apiResp.AssumeInitialArn
	}

	if apiResp.ProdEnvName != nil {
		remote.ProdEnvName = apiResp.ProdEnvName
	}

	// Map Jira config if present
	if apiResp.JiraConfigForAccessRequests != nil {
		remote.JiraConfigForAccessRequests = mapJiraAccessRequestConfig(apiResp.JiraConfigForAccessRequests)
	}

	return remote
}

// mapJiraAccessRequestConfig converts API Jira config to internal format
// The API schema is simpler (single project/issue-type) than the CLI structure
// (default/prod project pairs), so we map the API values to both default and prod
func mapJiraAccessRequestConfig(apiConfig *BackplaneApi.JiraAccessRequestConfig) *AccessRequestsJiraConfiguration {
	config := &AccessRequestsJiraConfiguration{
		ProjectToTransitionsNames: make(map[string]JiraTransitionsNamesForAccessRequests),
	}

	// Map project-key to both DefaultProject and ProdProject
	if apiConfig.ProjectKey != nil {
		config.DefaultProject = *apiConfig.ProjectKey
		config.ProdProject = *apiConfig.ProjectKey
	}

	// Map issue-type to both DefaultIssueType and ProdIssueType
	if apiConfig.IssueType != nil {
		config.DefaultIssueType = *apiConfig.IssueType
		config.ProdIssueType = *apiConfig.IssueType
	}

	// Map transition-states if present
	if apiConfig.TransitionStates != nil && len(*apiConfig.TransitionStates) > 0 {
		// Extract transition state names from the map
		// The API has a map[string]string containing transition names (human-readable JIRA state names) in both keys and values
		// We need to populate ProjectToTransitionsNames with transition names

		transitionMap := *apiConfig.TransitionStates

		// Try to extract known transition states
		transitions := JiraTransitionsNamesForAccessRequests{}

		// Look for common transition state names in the map keys
		if val, ok := transitionMap["approved"]; ok {
			transitions.OnApproval = val
		}
		if val, ok := transitionMap["in-progress"]; ok {
			transitions.OnCreation = val
		}
		if val, ok := transitionMap["rejected"]; ok {
			transitions.OnError = val
		}

		// Add the transitions for the project key only if at least one field is populated
		if apiConfig.ProjectKey != nil && (transitions.OnApproval != "" || transitions.OnCreation != "" || transitions.OnError != "") {
			config.ProjectToTransitionsNames[*apiConfig.ProjectKey] = transitions
		}
	}

	return config
}
