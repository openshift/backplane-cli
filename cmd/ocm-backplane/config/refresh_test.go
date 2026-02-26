package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/openshift/backplane-cli/pkg/cli/config"
	configMocks "github.com/openshift/backplane-cli/pkg/cli/config/mocks"
	"github.com/openshift/backplane-cli/pkg/ocm"
	ocmMock "github.com/openshift/backplane-cli/pkg/ocm/mocks"
)

var _ = Describe("refresh command", func() {
	var (
		mockCtrl         *gomock.Controller
		mockOcmInterface *ocmMock.MockOCMInterface
		mockAPIClient    *configMocks.MockConfigAPIClient

		tempDir        string
		configFile     string
		originalClient config.ConfigAPIClient
		originalOCM    ocm.OCMInterface
		cmd            *cobra.Command
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockOcmInterface = ocmMock.NewMockOCMInterface(mockCtrl)
		mockAPIClient = configMocks.NewMockConfigAPIClient(mockCtrl)

		// Save original API client and OCM interface
		originalClient = config.DefaultAPIClient
		originalOCM = ocm.DefaultOCMInterface

		// Create temp directory for config file
		var err error
		tempDir, err = os.MkdirTemp("", "refresh-test-*")
		Expect(err).To(BeNil())

		configFile = filepath.Join(tempDir, "config.json")
		os.Setenv("BACKPLANE_CONFIG", configFile)

		// Reset viper
		viper.Reset()

		// Set up OCM mock
		ocm.DefaultOCMInterface = mockOcmInterface

		// Set up config API client mock
		config.DefaultAPIClient = mockAPIClient

		// Create the refresh command
		cmd = newRefreshCmd()
	})

	AfterEach(func() {
		os.Unsetenv("BACKPLANE_CONFIG")
		os.RemoveAll(tempDir)
		viper.Reset()
		config.DefaultAPIClient = originalClient
		ocm.DefaultOCMInterface = originalOCM
		mockCtrl.Finish()
	})

	Context("when refreshing configuration successfully", func() {
		It("should fetch remote config and save to file", func() {
			// Set up existing config file with user settings
			existingConfig := `{
				"proxy-url": ["http://user-proxy:3128"],
				"session-dir": "/custom/session"
			}`
			err := os.WriteFile(configFile, []byte(existingConfig), 0600)
			Expect(err).To(BeNil())

			// Set up backplane URL
			ocmEnv, _ := cmv1.NewEnvironment().BackplaneURL("https://backplane.example.com").Build()
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()

			// Mock OCM token
			token := "test-token-12345"
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&token, nil)

			// Mock remote config response
			jiraBaseURL := "https://jira.example.com"
			assumeArn := "arn:aws:iam::123456789:role/Test-Role"
			prodEnv := "production-test"

			remoteConfig := &config.RemoteConfig{
				JiraBaseURL:      &jiraBaseURL,
				AssumeInitialArn: &assumeArn,
				ProdEnvName:      &prodEnv,
				JiraConfigForAccessRequests: &config.AccessRequestsJiraConfiguration{
					DefaultProject:   "TEST",
					DefaultIssueType: "Story",
					ProdProject:      "PROD",
					ProdIssueType:    "Incident",
					ProjectToTransitionsNames: map[string]config.JiraTransitionsNamesForAccessRequests{
						"TEST": {
							OnCreation: "In Progress",
							OnApproval: "Done",
							OnError:    "Closed",
						},
					},
				},
			}

			mockAPIClient.EXPECT().
				FetchRemoteConfig(gomock.Any()).
				Return(remoteConfig, nil)

			// Execute command
			err = cmd.Execute()
			Expect(err).To(BeNil())

			// Verify config file was updated
			fileContent, err := os.ReadFile(configFile)
			Expect(err).To(BeNil())

			// Verify server-managed values were written
			Expect(string(fileContent)).To(ContainSubstring("https://jira.example.com"))
			Expect(string(fileContent)).To(ContainSubstring("arn:aws:iam::123456789:role/Test-Role"))
			Expect(string(fileContent)).To(ContainSubstring("production-test"))
			Expect(string(fileContent)).To(ContainSubstring("TEST"))

			// Verify user settings were preserved
			Expect(string(fileContent)).To(ContainSubstring("http://user-proxy:3128"))
			Expect(string(fileContent)).To(ContainSubstring("/custom/session"))
		})

		It("should create config file if it doesn't exist", func() {
			// No existing config file

			// Set up backplane URL
			ocmEnv, _ := cmv1.NewEnvironment().BackplaneURL("https://backplane.example.com").Build()
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()

			// Mock OCM token
			token := "test-token-12345"
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&token, nil)

			// Mock remote config response
			jiraBaseURL := "https://jira.example.com"
			remoteConfig := &config.RemoteConfig{
				JiraBaseURL: &jiraBaseURL,
			}

			mockAPIClient.EXPECT().
				FetchRemoteConfig(gomock.Any()).
				Return(remoteConfig, nil)

			// Execute command
			err := cmd.Execute()
			Expect(err).To(BeNil())

			// Verify config file was created
			_, err = os.Stat(configFile)
			Expect(err).To(BeNil())
		})

		It("should overwrite existing server-managed values", func() {
			// Set up existing config file with old server values
			existingConfig := `{
				"jira-base-url": "https://old-jira.example.com",
				"assume-initial-arn": "arn:aws:iam::111111111:role/Old-Role",
				"prod-env-name": "old-production",
				"proxy-url": ["http://user-proxy:3128"]
			}`
			err := os.WriteFile(configFile, []byte(existingConfig), 0600)
			Expect(err).To(BeNil())

			// Set up backplane URL
			ocmEnv, _ := cmv1.NewEnvironment().BackplaneURL("https://backplane.example.com").Build()
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()

			// Mock OCM token
			token := "test-token-12345"
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&token, nil)

			// Mock remote config response with NEW values
			newJiraBaseURL := "https://new-jira.example.com"
			newAssumeArn := "arn:aws:iam::999999999:role/New-Role"
			newProdEnv := "new-production"

			remoteConfig := &config.RemoteConfig{
				JiraBaseURL:      &newJiraBaseURL,
				AssumeInitialArn: &newAssumeArn,
				ProdEnvName:      &newProdEnv,
			}

			mockAPIClient.EXPECT().
				FetchRemoteConfig(gomock.Any()).
				Return(remoteConfig, nil)

			// Execute command
			err = cmd.Execute()
			Expect(err).To(BeNil())

			// Verify config file has NEW values
			fileContent, err := os.ReadFile(configFile)
			Expect(err).To(BeNil())

			Expect(string(fileContent)).To(ContainSubstring("new-jira.example.com"))
			Expect(string(fileContent)).To(ContainSubstring("arn:aws:iam::999999999:role/New-Role"))
			Expect(string(fileContent)).To(ContainSubstring("new-production"))

			// Verify OLD values are not present
			Expect(string(fileContent)).NotTo(ContainSubstring("old-jira.example.com"))
			Expect(string(fileContent)).NotTo(ContainSubstring("arn:aws:iam::111111111:role/Old-Role"))
			Expect(string(fileContent)).NotTo(ContainSubstring("old-production"))

			// Verify user settings were preserved
			Expect(string(fileContent)).To(ContainSubstring("http://user-proxy:3128"))
		})

		It("should not leak viper defaults into the config file", func() {
			// Set up existing config file with minimal user settings (no govcloud)
			existingConfig := `{
				"proxy-url": ["http://user-proxy:3128"]
			}`
			err := os.WriteFile(configFile, []byte(existingConfig), 0600)
			Expect(err).To(BeNil())

			// Set up backplane URL
			ocmEnv, _ := cmv1.NewEnvironment().BackplaneURL("https://backplane.example.com").Build()
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()

			// Mock OCM token
			token := "test-token-12345"
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&token, nil)

			// Mock remote config response
			jiraBaseURL := "https://jira.example.com"
			remoteConfig := &config.RemoteConfig{
				JiraBaseURL: &jiraBaseURL,
			}

			mockAPIClient.EXPECT().
				FetchRemoteConfig(gomock.Any()).
				Return(remoteConfig, nil)

			// Execute command
			err = cmd.Execute()
			Expect(err).To(BeNil())

			// Read the written config file
			fileContent, err := os.ReadFile(configFile)
			Expect(err).To(BeNil())

			// Verify server-managed value was written
			Expect(string(fileContent)).To(ContainSubstring("https://jira.example.com"))

			// Verify user setting was preserved
			Expect(string(fileContent)).To(ContainSubstring("http://user-proxy:3128"))

			// CRITICAL: Verify that viper defaults (like govcloud) did NOT leak into the file
			Expect(string(fileContent)).NotTo(ContainSubstring("govcloud"))
		})
	})

	Context("when errors occur", func() {
		It("should fail if config cannot be loaded", func() {
			// Save and unset BACKPLANE_URL to ensure config load fails
			originalBackplaneURL, wasSet := os.LookupEnv("BACKPLANE_URL")
			os.Unsetenv("BACKPLANE_URL")
			defer func() {
				if wasSet {
					os.Setenv("BACKPLANE_URL", originalBackplaneURL)
				}
			}()

			// Don't set BACKPLANE_URL env var, and no OCM env
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(nil, fmt.Errorf("no OCM environment"))

			// Execute command
			err := cmd.Execute()
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to load current configuration"))
		})

		It("should fail if OCM access token cannot be obtained", func() {
			// Set up backplane URL
			ocmEnv, _ := cmv1.NewEnvironment().BackplaneURL("https://backplane.example.com").Build()
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()

			// Mock OCM token failure
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(nil, fmt.Errorf("token error"))

			// Execute command
			err := cmd.Execute()
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to get OCM access token"))
		})

		It("should fail if remote config fetch fails", func() {
			// Set up backplane URL
			ocmEnv, _ := cmv1.NewEnvironment().BackplaneURL("https://backplane.example.com").Build()
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()

			// Mock OCM token
			token := "test-token-12345"
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&token, nil)

			// Mock remote config fetch failure
			mockAPIClient.EXPECT().
				FetchRemoteConfig(gomock.Any()).
				Return(nil, fmt.Errorf("API error"))

			// Execute command
			err := cmd.Execute()
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to fetch configuration from server"))
		})

		It("should warn if no configuration values are returned", func() {
			// Capture logger output
			var logBuffer bytes.Buffer
			originalOutput := logger.StandardLogger().Out
			logger.SetOutput(&logBuffer)
			DeferCleanup(func() {
				logger.SetOutput(originalOutput)
			})

			// Set up backplane URL
			ocmEnv, _ := cmv1.NewEnvironment().BackplaneURL("https://backplane.example.com").Build()
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()

			// Mock OCM token
			token := "test-token-12345"
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&token, nil)

			// Mock remote config response with no values
			remoteConfig := &config.RemoteConfig{}

			mockAPIClient.EXPECT().
				FetchRemoteConfig(gomock.Any()).
				Return(remoteConfig, nil)

			// Execute command - should still succeed but log warning
			err := cmd.Execute()
			Expect(err).To(BeNil())

			// Verify warning was logged
			logOutput := logBuffer.String()
			Expect(logOutput).To(ContainSubstring("No configuration values were returned from the server"))
		})
	})
})
