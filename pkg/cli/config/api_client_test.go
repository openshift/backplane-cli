package config

import (
	"testing"

	BackplaneApi "github.com/openshift/backplane-api/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapAPIResponseToRemoteConfig(t *testing.T) {
	t.Run("maps all fields when present", func(t *testing.T) {
		jiraURL := "https://jira.example.com"
		arnValue := "arn:aws:iam::123456789:role/Test-Role"
		prodEnv := "production"
		projectKey := "TESTPROJ"
		issueType := "Story"

		input := &BackplaneApi.ClientConfig{
			JiraBaseUrl:      &jiraURL,
			AssumeInitialArn: &arnValue,
			ProdEnvName:      &prodEnv,
			JiraConfigForAccessRequests: &BackplaneApi.JiraAccessRequestConfig{
				ProjectKey: &projectKey,
				IssueType:  &issueType,
			},
		}

		result := mapAPIResponseToRemoteConfig(input)

		require.NotNil(t, result)
		require.NotNil(t, result.JiraBaseURL)
		assert.Equal(t, jiraURL, *result.JiraBaseURL)
		require.NotNil(t, result.AssumeInitialArn)
		assert.Equal(t, arnValue, *result.AssumeInitialArn)
		require.NotNil(t, result.ProdEnvName)
		assert.Equal(t, prodEnv, *result.ProdEnvName)
		require.NotNil(t, result.JiraConfigForAccessRequests)
		assert.Equal(t, projectKey, result.JiraConfigForAccessRequests.DefaultProject)
	})

	t.Run("handles nil JiraBaseUrl", func(t *testing.T) {
		input := &BackplaneApi.ClientConfig{
			JiraBaseUrl: nil,
		}

		result := mapAPIResponseToRemoteConfig(input)

		assert.NotNil(t, result)
		assert.Nil(t, result.JiraBaseURL)
	})

	t.Run("handles nil AssumeInitialArn", func(t *testing.T) {
		input := &BackplaneApi.ClientConfig{
			AssumeInitialArn: nil,
		}

		result := mapAPIResponseToRemoteConfig(input)

		assert.NotNil(t, result)
		assert.Nil(t, result.AssumeInitialArn)
	})

	t.Run("handles nil ProdEnvName", func(t *testing.T) {
		input := &BackplaneApi.ClientConfig{
			ProdEnvName: nil,
		}

		result := mapAPIResponseToRemoteConfig(input)

		assert.NotNil(t, result)
		assert.Nil(t, result.ProdEnvName)
	})

	t.Run("handles nil JiraConfigForAccessRequests", func(t *testing.T) {
		input := &BackplaneApi.ClientConfig{
			JiraConfigForAccessRequests: nil,
		}

		result := mapAPIResponseToRemoteConfig(input)

		assert.NotNil(t, result)
		assert.Nil(t, result.JiraConfigForAccessRequests)
	})

	t.Run("handles completely empty config", func(t *testing.T) {
		input := &BackplaneApi.ClientConfig{}

		result := mapAPIResponseToRemoteConfig(input)

		assert.NotNil(t, result)
		assert.Nil(t, result.JiraBaseURL)
		assert.Nil(t, result.AssumeInitialArn)
		assert.Nil(t, result.ProdEnvName)
		assert.Nil(t, result.JiraConfigForAccessRequests)
	})

	t.Run("calls mapJiraAccessRequestConfig when Jira config is present", func(t *testing.T) {
		projectKey := "TEST"
		issueType := "Bug"
		transitionStates := map[string]string{
			"approved": "Done",
		}

		input := &BackplaneApi.ClientConfig{
			JiraConfigForAccessRequests: &BackplaneApi.JiraAccessRequestConfig{
				ProjectKey:       &projectKey,
				IssueType:        &issueType,
				TransitionStates: &transitionStates,
			},
		}

		result := mapAPIResponseToRemoteConfig(input)

		assert.NotNil(t, result)
		assert.NotNil(t, result.JiraConfigForAccessRequests)
		assert.Equal(t, "TEST", result.JiraConfigForAccessRequests.DefaultProject)
		assert.Equal(t, "Bug", result.JiraConfigForAccessRequests.DefaultIssueType)
	})
}

func TestMapJiraAccessRequestConfig(t *testing.T) {
	t.Run("maps project key to both default and prod", func(t *testing.T) {
		projectKey := "TESTPROJECT"

		input := &BackplaneApi.JiraAccessRequestConfig{
			ProjectKey: &projectKey,
		}

		result := mapJiraAccessRequestConfig(input)

		assert.NotNil(t, result)
		assert.Equal(t, "TESTPROJECT", result.DefaultProject)
		assert.Equal(t, "TESTPROJECT", result.ProdProject)
		assert.NotNil(t, result.ProjectToTransitionsNames)
	})

	t.Run("maps issue type to both default and prod", func(t *testing.T) {
		issueType := "Story"

		input := &BackplaneApi.JiraAccessRequestConfig{
			IssueType: &issueType,
		}

		result := mapJiraAccessRequestConfig(input)

		assert.NotNil(t, result)
		assert.Equal(t, "Story", result.DefaultIssueType)
		assert.Equal(t, "Story", result.ProdIssueType)
	})

	t.Run("maps all basic fields together", func(t *testing.T) {
		projectKey := "MYPROJECT"
		issueType := "Task"

		input := &BackplaneApi.JiraAccessRequestConfig{
			ProjectKey: &projectKey,
			IssueType:  &issueType,
		}

		result := mapJiraAccessRequestConfig(input)

		assert.Equal(t, "MYPROJECT", result.DefaultProject)
		assert.Equal(t, "MYPROJECT", result.ProdProject)
		assert.Equal(t, "Task", result.DefaultIssueType)
		assert.Equal(t, "Task", result.ProdIssueType)
	})

	t.Run("handles nil project key - returns nil for empty config", func(t *testing.T) {
		input := &BackplaneApi.JiraAccessRequestConfig{
			ProjectKey: nil,
		}

		result := mapJiraAccessRequestConfig(input)

		assert.Nil(t, result, "should return nil when config has no meaningful data")
	})

	t.Run("handles nil issue type - returns nil for empty config", func(t *testing.T) {
		input := &BackplaneApi.JiraAccessRequestConfig{
			IssueType: nil,
		}

		result := mapJiraAccessRequestConfig(input)

		assert.Nil(t, result, "should return nil when config has no meaningful data")
	})

	t.Run("maps all transition states correctly", func(t *testing.T) {
		projectKey := "TEST"
		transitionStates := map[string]string{
			"approved":    "Done",
			"in-progress": "In Progress",
			"rejected":    "Closed",
		}

		input := &BackplaneApi.JiraAccessRequestConfig{
			ProjectKey:       &projectKey,
			TransitionStates: &transitionStates,
		}

		result := mapJiraAccessRequestConfig(input)

		assert.NotNil(t, result)
		require.Contains(t, result.ProjectToTransitionsNames, "TEST")

		transitions := result.ProjectToTransitionsNames["TEST"]
		assert.Equal(t, "Done", transitions.OnApproval)
		assert.Equal(t, "In Progress", transitions.OnCreation)
		assert.Equal(t, "Closed", transitions.OnError)
	})

	t.Run("handles partial transition states - only approved", func(t *testing.T) {
		projectKey := "PARTIAL"
		transitionStates := map[string]string{
			"approved": "Done",
		}

		input := &BackplaneApi.JiraAccessRequestConfig{
			ProjectKey:       &projectKey,
			TransitionStates: &transitionStates,
		}

		result := mapJiraAccessRequestConfig(input)

		require.Contains(t, result.ProjectToTransitionsNames, "PARTIAL")
		transitions := result.ProjectToTransitionsNames["PARTIAL"]
		assert.Equal(t, "Done", transitions.OnApproval)
		assert.Empty(t, transitions.OnCreation)
		assert.Empty(t, transitions.OnError)
	})

	t.Run("handles partial transition states - only in-progress", func(t *testing.T) {
		projectKey := "PARTIAL2"
		transitionStates := map[string]string{
			"in-progress": "Working",
		}

		input := &BackplaneApi.JiraAccessRequestConfig{
			ProjectKey:       &projectKey,
			TransitionStates: &transitionStates,
		}

		result := mapJiraAccessRequestConfig(input)

		require.Contains(t, result.ProjectToTransitionsNames, "PARTIAL2")
		transitions := result.ProjectToTransitionsNames["PARTIAL2"]
		assert.Empty(t, transitions.OnApproval)
		assert.Equal(t, "Working", transitions.OnCreation)
		assert.Empty(t, transitions.OnError)
	})

	t.Run("handles partial transition states - only rejected", func(t *testing.T) {
		projectKey := "PARTIAL3"
		transitionStates := map[string]string{
			"rejected": "Failed",
		}

		input := &BackplaneApi.JiraAccessRequestConfig{
			ProjectKey:       &projectKey,
			TransitionStates: &transitionStates,
		}

		result := mapJiraAccessRequestConfig(input)

		require.Contains(t, result.ProjectToTransitionsNames, "PARTIAL3")
		transitions := result.ProjectToTransitionsNames["PARTIAL3"]
		assert.Empty(t, transitions.OnApproval)
		assert.Empty(t, transitions.OnCreation)
		assert.Equal(t, "Failed", transitions.OnError)
	})

	t.Run("ignores unknown transition state keys", func(t *testing.T) {
		projectKey := "UNKNOWN"
		transitionStates := map[string]string{
			"unknown-key":   "SomeValue",
			"another-key":   "AnotherValue",
			"approved":      "Done",
		}

		input := &BackplaneApi.JiraAccessRequestConfig{
			ProjectKey:       &projectKey,
			TransitionStates: &transitionStates,
		}

		result := mapJiraAccessRequestConfig(input)

		require.Contains(t, result.ProjectToTransitionsNames, "UNKNOWN")
		transitions := result.ProjectToTransitionsNames["UNKNOWN"]
		assert.Equal(t, "Done", transitions.OnApproval)
		assert.Empty(t, transitions.OnCreation)
		assert.Empty(t, transitions.OnError)
	})

	t.Run("handles nil transition states", func(t *testing.T) {
		projectKey := "NOTRANS"

		input := &BackplaneApi.JiraAccessRequestConfig{
			ProjectKey:       &projectKey,
			TransitionStates: nil,
		}

		result := mapJiraAccessRequestConfig(input)

		assert.NotNil(t, result)
		assert.NotContains(t, result.ProjectToTransitionsNames, "NOTRANS")
	})

	t.Run("handles empty transition states map", func(t *testing.T) {
		projectKey := "EMPTY"
		transitionStates := map[string]string{}

		input := &BackplaneApi.JiraAccessRequestConfig{
			ProjectKey:       &projectKey,
			TransitionStates: &transitionStates,
		}

		result := mapJiraAccessRequestConfig(input)

		assert.NotNil(t, result)
		assert.NotContains(t, result.ProjectToTransitionsNames, "EMPTY")
	})

	t.Run("does not add to ProjectToTransitionsNames if project key is nil - returns nil", func(t *testing.T) {
		transitionStates := map[string]string{
			"approved": "Done",
		}

		input := &BackplaneApi.JiraAccessRequestConfig{
			ProjectKey:       nil,
			TransitionStates: &transitionStates,
		}

		result := mapJiraAccessRequestConfig(input)

		assert.Nil(t, result, "should return nil when transition states exist but project key is nil (no meaningful data)")
	})

	t.Run("complete config with all fields", func(t *testing.T) {
		projectKey := "COMPLETE"
		issueType := "Epic"
		transitionStates := map[string]string{
			"approved":    "Completed",
			"in-progress": "In Development",
			"rejected":    "Cancelled",
		}

		input := &BackplaneApi.JiraAccessRequestConfig{
			ProjectKey:       &projectKey,
			IssueType:        &issueType,
			TransitionStates: &transitionStates,
		}

		result := mapJiraAccessRequestConfig(input)

		assert.Equal(t, "COMPLETE", result.DefaultProject)
		assert.Equal(t, "COMPLETE", result.ProdProject)
		assert.Equal(t, "Epic", result.DefaultIssueType)
		assert.Equal(t, "Epic", result.ProdIssueType)

		require.Contains(t, result.ProjectToTransitionsNames, "COMPLETE")
		transitions := result.ProjectToTransitionsNames["COMPLETE"]
		assert.Equal(t, "Completed", transitions.OnApproval)
		assert.Equal(t, "In Development", transitions.OnCreation)
		assert.Equal(t, "Cancelled", transitions.OnError)
	})

	t.Run("empty config returns nil to avoid overwriting local config", func(t *testing.T) {
		input := &BackplaneApi.JiraAccessRequestConfig{}

		result := mapJiraAccessRequestConfig(input)

		assert.Nil(t, result, "should return nil for completely empty config to prevent overwriting existing local values with zero-values")
	})
}

// Note: FetchRemoteConfig is tested indirectly through the refresh_test.go integration tests
// Direct unit testing would require regenerating mocks after the /config endpoint was added
