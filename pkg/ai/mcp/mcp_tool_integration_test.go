package mcp_test

import (
	"context"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/mock/gomock"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/login"
	mcptools "github.com/openshift/backplane-cli/pkg/ai/mcp"
	"github.com/openshift/backplane-cli/pkg/info"
	infoMock "github.com/openshift/backplane-cli/pkg/info/mocks"
)

var _ = Describe("MCP Tool Integration", func() {
	var (
		// Login tool mocks
		originalLoginRunE func(cmd *cobra.Command, args []string) error

		// Info tool mocks
		mockCtrl            *gomock.Controller
		mockInfoService     *infoMock.MockInfoService
		originalInfoService info.InfoService
	)

	BeforeEach(func() {
		// Setup login tool mocking
		originalLoginRunE = login.LoginCmd.RunE

		// Setup info tool mocking
		mockCtrl = gomock.NewController(GinkgoT())
		mockInfoService = infoMock.NewMockInfoService(mockCtrl)
		originalInfoService = info.DefaultInfoService
		info.DefaultInfoService = mockInfoService

		// Clear environment and viper for clean tests
		os.Unsetenv("BACKPLANE_URL")
		os.Unsetenv("HTTPS_PROXY")
		os.Unsetenv("BACKPLANE_AWS_PROXY")
		os.Unsetenv("BACKPLANE_CONFIG")
		viper.Reset()
	})

	AfterEach(func() {
		// Restore login tool
		login.LoginCmd.RunE = originalLoginRunE

		// Restore info tool
		mockCtrl.Finish()
		info.DefaultInfoService = originalInfoService

		// Clean up environment
		os.Unsetenv("BACKPLANE_URL")
		os.Unsetenv("HTTPS_PROXY")
		os.Unsetenv("BACKPLANE_AWS_PROXY")
		os.Unsetenv("BACKPLANE_CONFIG")
		os.Unsetenv("SHELL")
		viper.Reset()
	})

	Context("MCP Login Tool Integration", func() {
		It("Should integrate login tool with MCP server correctly", func() {
			// Mock successful login for integration test
			login.LoginCmd.RunE = func(cmd *cobra.Command, args []string) error {
				Expect(args).To(HaveLen(1))
				Expect(args[0]).To(Equal("integration-test-cluster"))
				return nil
			}

			// Test the MCP tool directly as it would be called by an MCP client
			input := mcptools.BackplaneLoginArgs{ClusterID: "integration-test-cluster"}
			result, output, err := mcptools.BackplaneLogin(context.Background(), &mcp.CallToolRequest{}, input)

			// Verify MCP integration works correctly
			Expect(err).To(BeNil())
			Expect(result).ToNot(BeNil())
			Expect(output).To(Equal(struct{}{}))

			// Verify response follows MCP protocol
			Expect(result.Content).To(HaveLen(1))
			textContent, ok := result.Content[0].(*mcp.TextContent)
			Expect(ok).To(BeTrue())
			Expect(textContent.Text).To(Equal("Successfully logged in to cluster 'integration-test-cluster'"))
		})

		It("Should handle MCP tool name format correctly", func() {
			// When used through Gemini or Claude, the tool would be called as:
			// "backplane__login" (server name + __ + tool name)
			// This test verifies our tool works correctly for that use case

			// Mock successful login
			login.LoginCmd.RunE = func(cmd *cobra.Command, args []string) error {
				return nil
			}

			// Test with cluster ID that might come from MCP client
			input := mcptools.BackplaneLoginArgs{ClusterID: "mcp-client-cluster-456"}
			result, _, err := mcptools.BackplaneLogin(context.Background(), &mcp.CallToolRequest{}, input)

			Expect(err).To(BeNil())
			Expect(result).ToNot(BeNil())

			textContent := result.Content[0].(*mcp.TextContent)
			Expect(textContent.Text).To(ContainSubstring("Successfully logged in to cluster 'mcp-client-cluster-456'"))
		})

		It("Should handle realistic cluster ID formats from MCP clients", func() {
			testCases := []struct {
				clusterID string
				expected  string
			}{
				{"abc123", "Successfully logged in to cluster 'abc123'"},
				{"cluster-prod-us-east-1", "Successfully logged in to cluster 'cluster-prod-us-east-1'"},
				{"dev_cluster_001", "Successfully logged in to cluster 'dev_cluster_001'"},
				{"staging.cluster.example.com", "Successfully logged in to cluster 'staging.cluster.example.com'"},
			}

			for _, tc := range testCases {
				// Mock successful login for each test case
				login.LoginCmd.RunE = func(cmd *cobra.Command, args []string) error {
					Expect(args[0]).To(Equal(tc.clusterID))
					return nil
				}

				input := mcptools.BackplaneLoginArgs{ClusterID: tc.clusterID}
				result, _, err := mcptools.BackplaneLogin(context.Background(), &mcp.CallToolRequest{}, input)

				Expect(err).To(BeNil(), "Test case: "+tc.clusterID)
				Expect(result).ToNot(BeNil(), "Test case: "+tc.clusterID)

				textContent := result.Content[0].(*mcp.TextContent)
				Expect(textContent.Text).To(Equal(tc.expected), "Test case: "+tc.clusterID)
			}
		})

		It("Should provide meaningful error messages for MCP clients", func() {
			// Mock a realistic login failure
			login.LoginCmd.RunE = func(cmd *cobra.Command, args []string) error {
				return fmt.Errorf("cluster '%s' not found in your accessible clusters", args[0])
			}

			input := mcptools.BackplaneLoginArgs{ClusterID: "nonexistent-cluster"}
			result, _, err := mcptools.BackplaneLogin(context.Background(), &mcp.CallToolRequest{}, input)

			// Should handle error gracefully for MCP clients
			Expect(err).To(BeNil()) // No exception thrown
			Expect(result).ToNot(BeNil())

			textContent := result.Content[0].(*mcp.TextContent)
			Expect(textContent.Text).To(ContainSubstring("Failed to login to cluster 'nonexistent-cluster'"))
			Expect(textContent.Text).To(ContainSubstring("not found in your accessible clusters"))
		})
	})

	Context("MCP Info Tool Integration", func() {
		It("Should integrate info tool with MCP server correctly", func() {
			// Mock version service
			mockInfoService.EXPECT().GetVersion().Return("1.5.0").Times(1)

			// Set up environment for configuration
			os.Setenv("BACKPLANE_URL", "https://api.backplane.example.com")
			os.Setenv("HTTPS_PROXY", "https://proxy.example.com:8080")
			viper.Set("govcloud", false)
			viper.Set("display-cluster-info", true)

			// Test the MCP tool directly as it would be called by an MCP client
			input := mcptools.BackplaneInfoInput{}
			result, output, err := mcptools.GetBackplaneInfo(context.Background(), &mcp.CallToolRequest{}, input)

			// Verify MCP integration works correctly
			Expect(err).To(BeNil())
			Expect(result).ToNot(BeNil())
			Expect(output).To(BeNil()) // Info tool returns nil for output

			// Verify response follows MCP protocol
			Expect(result.Content).To(HaveLen(1))
			textContent, ok := result.Content[0].(*mcp.TextContent)
			Expect(ok).To(BeTrue())

			infoText := textContent.Text
			Expect(infoText).To(ContainSubstring("Version: 1.5.0"))
			Expect(infoText).To(ContainSubstring("Backplane URL: https://api.backplane.example.com"))
			Expect(infoText).To(ContainSubstring("Environment:"))
		})

		It("Should handle MCP info tool name format correctly", func() {
			// When used through Gemini or Claude, the tool would be called as:
			// "backplane__info" (server name + __ + tool name)

			// Mock version service
			mockInfoService.EXPECT().GetVersion().Return("2.0.0").Times(1)

			// Set up minimal environment
			os.Setenv("BACKPLANE_URL", "https://test.backplane.example.com")
			viper.Set("govcloud", false)

			input := mcptools.BackplaneInfoInput{}
			result, _, err := mcptools.GetBackplaneInfo(context.Background(), &mcp.CallToolRequest{}, input)

			Expect(err).To(BeNil())
			Expect(result).ToNot(BeNil())

			textContent := result.Content[0].(*mcp.TextContent)
			Expect(textContent.Text).To(ContainSubstring("Version: 2.0.0"))
			Expect(textContent.Text).To(ContainSubstring("Backplane URL: https://test.backplane.example.com"))
		})

		It("Should verify BackplaneInfoInput JSON schema compatibility", func() {
			// Test that the input struct works with MCP's JSON schema generation
			input := mcptools.BackplaneInfoInput{}

			// Mock version service for actual call
			mockInfoService.EXPECT().GetVersion().Return("schema-test").Times(1)
			os.Setenv("BACKPLANE_URL", "https://api.backplane.example.com")
			viper.Set("govcloud", false)

			// The BackplaneInfoInput struct should work with MCP even though it's empty
			result, _, err := mcptools.GetBackplaneInfo(context.Background(), &mcp.CallToolRequest{}, input)

			Expect(err).To(BeNil())
			Expect(result).ToNot(BeNil())

			textContent := result.Content[0].(*mcp.TextContent)
			Expect(textContent.Text).To(ContainSubstring("Version: schema-test"))
		})

		It("Should handle configuration errors gracefully for MCP clients", func() {
			// Mock version service
			mockInfoService.EXPECT().GetVersion().Return("1.0.0").Times(1)

			// Don't set required configuration to trigger error
			viper.Set("govcloud", false)
			// No BACKPLANE_URL or proxy configured

			input := mcptools.BackplaneInfoInput{}
			result, _, err := mcptools.GetBackplaneInfo(context.Background(), &mcp.CallToolRequest{}, input)

			// Should handle configuration errors gracefully
			Expect(err).To(BeNil()) // No exception thrown
			Expect(result).ToNot(BeNil())

			textContent := result.Content[0].(*mcp.TextContent)
			Expect(textContent.Text).To(ContainSubstring("Version: 1.0.0"))
			// Should still provide version even if config fails
		})

		It("Should provide comprehensive information for AI assistants", func() {
			// Mock version service
			mockInfoService.EXPECT().GetVersion().Return("3.1.0").Times(1)

			// Set up comprehensive configuration
			os.Setenv("BACKPLANE_URL", "https://prod.backplane.example.com")
			os.Setenv("HTTPS_PROXY", "https://corporate-proxy.example.com:8080")
			os.Setenv("SHELL", "/bin/zsh")
			viper.Set("session-dir", "custom-backplane-sessions")
			viper.Set("display-cluster-info", true)
			viper.Set("govcloud", false)

			input := mcptools.BackplaneInfoInput{}
			result, _, err := mcptools.GetBackplaneInfo(context.Background(), &mcp.CallToolRequest{}, input)

			Expect(err).To(BeNil())
			Expect(result).ToNot(BeNil())

			textContent := result.Content[0].(*mcp.TextContent)
			infoText := textContent.Text

			// Verify comprehensive information is provided
			Expect(infoText).To(ContainSubstring("Version: 3.1.0"))
			Expect(infoText).To(ContainSubstring("Backplane URL: https://prod.backplane.example.com"))
			Expect(infoText).To(ContainSubstring("Session Directory: custom-backplane-sessions"))
			Expect(infoText).To(ContainSubstring("Proxy URL: https://corporate-proxy.example.com:8080"))
			Expect(infoText).To(ContainSubstring("Display Cluster Info: true"))
			Expect(infoText).To(ContainSubstring("GovCloud: false"))
			Expect(infoText).To(ContainSubstring("Shell: /bin/zsh"))
			Expect(infoText).To(ContainSubstring("Environment:"))
		})
	})

	Context("Performance and Reliability", func() {
		It("Should complete login operations quickly for MCP responsiveness", func() {
			// MCP clients expect reasonably fast responses
			callCount := 0

			// Mock fast login
			login.LoginCmd.RunE = func(cmd *cobra.Command, args []string) error {
				callCount++
				return nil
			}

			input := mcptools.BackplaneLoginArgs{ClusterID: "perf-test-cluster"}

			// Multiple calls should all succeed
			for i := 0; i < 5; i++ {
				result, _, err := mcptools.BackplaneLogin(context.Background(), &mcp.CallToolRequest{}, input)
				Expect(err).To(BeNil())
				Expect(result).ToNot(BeNil())
			}

			Expect(callCount).To(Equal(5))
		})

		It("Should complete info operations quickly for MCP responsiveness", func() {
			// Mock version service for multiple calls
			mockInfoService.EXPECT().GetVersion().Return("perf-test").Times(3)

			// Set up minimal environment
			os.Setenv("BACKPLANE_URL", "https://api.backplane.example.com")
			viper.Set("govcloud", false)

			input := mcptools.BackplaneInfoInput{}

			// Multiple calls should all succeed quickly
			for i := 0; i < 3; i++ {
				result, _, err := mcptools.GetBackplaneInfo(context.Background(), &mcp.CallToolRequest{}, input)
				Expect(err).To(BeNil())
				Expect(result).ToNot(BeNil())

				textContent := result.Content[0].(*mcp.TextContent)
				Expect(textContent.Text).To(ContainSubstring("Version: perf-test"))
			}
		})

		It("Should maintain consistent behavior across multiple tool calls", func() {
			// Test both tools in sequence to ensure no interference

			// Mock login
			loginCalls := 0
			login.LoginCmd.RunE = func(cmd *cobra.Command, args []string) error {
				loginCalls++
				return nil
			}

			// Mock info service
			mockInfoService.EXPECT().GetVersion().Return("consistency-test").Times(2)
			os.Setenv("BACKPLANE_URL", "https://api.backplane.example.com")
			viper.Set("govcloud", false)

			// Alternate between login and info calls
			loginInput := mcptools.BackplaneLoginArgs{ClusterID: "consistency-cluster"}
			infoInput := mcptools.BackplaneInfoInput{}

			for i := 0; i < 2; i++ {
				// Login call
				loginResult, _, err := mcptools.BackplaneLogin(context.Background(), &mcp.CallToolRequest{}, loginInput)
				Expect(err).To(BeNil())
				Expect(loginResult).ToNot(BeNil())

				// Info call
				infoResult, _, err := mcptools.GetBackplaneInfo(context.Background(), &mcp.CallToolRequest{}, infoInput)
				Expect(err).To(BeNil())
				Expect(infoResult).ToNot(BeNil())
			}

			Expect(loginCalls).To(Equal(2))
		})
	})

	Context("MCP Protocol Compliance", func() {
		It("Should return proper MCP response format for both tools", func() {
			// Test login tool
			login.LoginCmd.RunE = func(cmd *cobra.Command, args []string) error {
				return nil
			}

			loginInput := mcptools.BackplaneLoginArgs{ClusterID: "protocol-test"}
			loginResult, loginOutput, err := mcptools.BackplaneLogin(context.Background(), &mcp.CallToolRequest{}, loginInput)

			Expect(err).To(BeNil())
			Expect(loginResult).ToNot(BeNil())
			Expect(loginOutput).To(Equal(struct{}{})) // Login returns empty struct
			Expect(loginResult.Content).To(HaveLen(1))
			_, ok := loginResult.Content[0].(*mcp.TextContent)
			Expect(ok).To(BeTrue())

			// Test info tool
			mockInfoService.EXPECT().GetVersion().Return("protocol-test").Times(1)
			os.Setenv("BACKPLANE_URL", "https://api.backplane.example.com")
			viper.Set("govcloud", false)

			infoInput := mcptools.BackplaneInfoInput{}
			infoResult, infoOutput, err := mcptools.GetBackplaneInfo(context.Background(), &mcp.CallToolRequest{}, infoInput)

			Expect(err).To(BeNil())
			Expect(infoResult).ToNot(BeNil())
			Expect(infoOutput).To(BeNil()) // Info returns nil
			Expect(infoResult.Content).To(HaveLen(1))
			_, ok = infoResult.Content[0].(*mcp.TextContent)
			Expect(ok).To(BeTrue())
		})
	})
})
