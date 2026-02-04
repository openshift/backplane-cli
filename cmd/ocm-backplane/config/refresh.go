package config

import (
	"context"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	BackplaneApi "github.com/openshift/backplane-api/pkg/client"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/info"
	"github.com/openshift/backplane-cli/pkg/ocm"
	logger "github.com/sirupsen/logrus"
)

func newRefreshCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "Refresh configuration from backplane-api server",
		Long: `Fetch the latest configuration values from the backplane-api server and update the local config file.

This command will overwrite the following server-managed configuration values:
  - jira-base-url
  - assume-initial-arn
  - prod-env-name
  - jira-config-for-access-requests

User-defined values (proxy-url, session-dir, etc.) will not be modified.`,
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE:         runRefresh,
	}

	return cmd
}

func runRefresh(cmd *cobra.Command, args []string) error {
	logger.Info("Refreshing configuration from backplane-api...")

	// Get current config to determine backplane URL
	bpConfig, err := config.GetBackplaneConfiguration()
	if err != nil {
		return fmt.Errorf("failed to load current configuration: %w", err)
	}

	// Get OCM access token
	token, err := ocm.DefaultOCMInterface.GetOCMAccessToken()
	if err != nil {
		return fmt.Errorf("failed to get OCM access token: %w", err)
	}

	// Create backplane API client
	client, err := BackplaneApi.NewClient(bpConfig.URL, func(c *BackplaneApi.Client) error {
		c.RequestEditors = append(c.RequestEditors, func(ctx context.Context, req *http.Request) error {
			req.Header.Add("Authorization", "Bearer "+*token)
			req.Header.Set("User-Agent", "backplane-cli"+info.Version)
			req.Header.Set("Backplane-Version", info.DefaultInfoService.GetVersion())
			return nil
		})
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create backplane API client: %w", err)
	}

	// Fetch fresh config from API
	remoteConfig, err := config.DefaultAPIClient.FetchRemoteConfig(client)
	if err != nil {
		return fmt.Errorf("failed to fetch configuration from server: %w", err)
	}

	// Get config file path
	configPath, err := config.GetConfigFilePath()
	if err != nil {
		return fmt.Errorf("failed to get config file path: %w", err)
	}

	// Reset viper to clear any defaults that were set by GetBackplaneConfiguration
	// This prevents defaults like 'govcloud' from leaking into the user's config file
	viper.Reset()

	// Load existing config file to preserve user settings
	viper.SetConfigFile(configPath)
	viper.SetConfigType("json")
	if err := viper.ReadInConfig(); err != nil {
		// If config doesn't exist, that's okay - we'll create it
		logger.Debugf("No existing config file found, will create new one")
	}

	// Update only server-managed values
	updated := false
	if remoteConfig.JiraBaseURL != nil {
		viper.Set(config.JiraBaseURLKey, *remoteConfig.JiraBaseURL)
		logger.Infof("Updated jira-base-url: %s", *remoteConfig.JiraBaseURL)
		updated = true
	}

	if remoteConfig.AssumeInitialArn != nil {
		viper.Set(config.AssumeInitialArnKey, *remoteConfig.AssumeInitialArn)
		logger.Infof("Updated assume-initial-arn: %s", *remoteConfig.AssumeInitialArn)
		updated = true
	}

	if remoteConfig.ProdEnvName != nil {
		viper.Set(config.ProdEnvNameKey, *remoteConfig.ProdEnvName)
		logger.Infof("Updated prod-env-name: %s", *remoteConfig.ProdEnvName)
		updated = true
	}

	if remoteConfig.JiraConfigForAccessRequests != nil {
		viper.Set(config.JiraConfigForAccessRequestsKey, remoteConfig.JiraConfigForAccessRequests)
		logger.Info("Updated jira-config-for-access-requests")
		updated = true
	}

	if !updated {
		logger.Warn("No configuration values were returned from the server")
	}

	// Write updated config to file
	if err := viper.WriteConfigAs(configPath); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("Configuration refreshed successfully and saved to %s\n", configPath)
	return nil
}
