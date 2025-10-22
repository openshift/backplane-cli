package mcp_test

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
	"go.uber.org/mock/gomock"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	mcptools "github.com/openshift/backplane-cli/pkg/ai/mcp"
	"github.com/openshift/backplane-cli/pkg/info"
	infoMock "github.com/openshift/backplane-cli/pkg/info/mocks"
)

var _ = Describe("BackplaneInfo", func() {
	var (
		mockCtrl            *gomock.Controller
		mockInfoService     *infoMock.MockInfoService
		originalInfoService info.InfoService
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockInfoService = infoMock.NewMockInfoService(mockCtrl)

		// Store original service to restore later
		originalInfoService = info.DefaultInfoService
		info.DefaultInfoService = mockInfoService

		// Clear all environment variables that might affect configuration
		_ = os.Unsetenv("BACKPLANE_URL")
		_ = os.Unsetenv("HTTPS_PROXY")
		_ = os.Unsetenv("BACKPLANE_AWS_PROXY")
		_ = os.Unsetenv("BACKPLANE_CONFIG")

		// Clear viper settings to ensure clean state
		viper.Reset()
	})

	AfterEach(func() {
		mockCtrl.Finish()

		// Restore original service
		info.DefaultInfoService = originalInfoService

		// Clean up environment variables
		_ = os.Unsetenv("BACKPLANE_URL")
		_ = os.Unsetenv("HTTPS_PROXY")
		_ = os.Unsetenv("BACKPLANE_AWS_PROXY")
		_ = os.Unsetenv("BACKPLANE_CONFIG")
		_ = os.Unsetenv("SHELL")

		// Clear viper settings
		viper.Reset()
	})

	Context("When getting backplane info", func() {
		It("Should return comprehensive info with all configuration details", func() {
			// Mock version service
			mockInfoService.EXPECT().GetVersion().Return("1.2.3").Times(1)

			// Set up environment for configuration
			_ = os.Setenv("BACKPLANE_URL", "https://api.backplane.example.com")
			_ = os.Setenv("HTTPS_PROXY", "https://proxy.example.com:8080")
			_ = os.Setenv("BACKPLANE_AWS_PROXY", "https://aws-proxy.example.com:8080")

			// Set up viper configuration
			viper.Set("session-dir", "custom-session")
			viper.Set("display-cluster-info", true)
			viper.Set("govcloud", false)

			// Execute the function
			result, _, err := mcptools.GetBackplaneInfo(context.Background(), &mcp.CallToolRequest{}, mcptools.BackplaneInfoInput{})

			// Verify results
			Expect(err).To(BeNil())
			Expect(result).ToNot(BeNil())
			Expect(result.Content).To(HaveLen(1))

			textContent, ok := result.Content[0].(*mcp.TextContent)
			Expect(ok).To(BeTrue())

			infoText := textContent.Text
			Expect(infoText).To(ContainSubstring("Version: 1.2.3"))
			Expect(infoText).To(ContainSubstring("Backplane URL: https://api.backplane.example.com"))
			Expect(infoText).To(ContainSubstring("Session Directory: custom-session"))
			Expect(infoText).To(ContainSubstring("Proxy URL: https://proxy.example.com:8080"))
			Expect(infoText).To(ContainSubstring("AWS Proxy: https://aws-proxy.example.com:8080"))
			Expect(infoText).To(ContainSubstring("Display Cluster Info: true"))
			Expect(infoText).To(ContainSubstring("GovCloud: false"))
			Expect(infoText).To(ContainSubstring("Environment:"))
		})

		It("Should handle missing proxy configuration gracefully", func() {
			// Mock version service
			mockInfoService.EXPECT().GetVersion().Return("2.0.0").Times(1)

			// Set up minimal environment for configuration
			_ = os.Setenv("BACKPLANE_URL", "https://api.backplane.example.com")
			// Explicitly set empty proxy to override system defaults
			_ = os.Setenv("HTTPS_PROXY", "")

			// Set up viper configuration
			viper.Set("govcloud", false)

			// Execute the function
			result, _, err := mcptools.GetBackplaneInfo(context.Background(), &mcp.CallToolRequest{}, mcptools.BackplaneInfoInput{})

			// Verify results
			Expect(err).To(BeNil())
			Expect(result).ToNot(BeNil())

			textContent := result.Content[0].(*mcp.TextContent)
			infoText := textContent.Text

			Expect(infoText).To(ContainSubstring("Version: 2.0.0"))
			Expect(infoText).To(ContainSubstring("Backplane URL: https://api.backplane.example.com"))
			Expect(infoText).To(ContainSubstring("Session Directory: backplane")) // default value
			// Don't check specific proxy values as they may be system-dependent
		})

		It("Should handle unknown version gracefully", func() {
			// Mock version service to return unknown
			mockInfoService.EXPECT().GetVersion().Return("unknown").Times(1)

			// Set up minimal environment
			_ = os.Setenv("BACKPLANE_URL", "https://api.backplane.example.com")
			viper.Set("govcloud", false)

			// Execute the function
			result, _, err := mcptools.GetBackplaneInfo(context.Background(), &mcp.CallToolRequest{}, mcptools.BackplaneInfoInput{})

			// Verify results
			Expect(err).To(BeNil())
			Expect(result).ToNot(BeNil())

			textContent := result.Content[0].(*mcp.TextContent)
			infoText := textContent.Text

			Expect(infoText).To(ContainSubstring("Version: unknown"))
		})

		It("Should include environment information", func() {
			// Mock version service
			mockInfoService.EXPECT().GetVersion().Return("3.0.0").Times(1)

			// Set up minimal environment
			_ = os.Setenv("BACKPLANE_URL", "https://api.backplane.example.com")
			_ = os.Setenv("SHELL", "/bin/zsh")
			viper.Set("govcloud", false)

			// Execute the function
			result, _, err := mcptools.GetBackplaneInfo(context.Background(), &mcp.CallToolRequest{}, mcptools.BackplaneInfoInput{})

			// Verify results
			Expect(err).To(BeNil())
			Expect(result).ToNot(BeNil())

			textContent := result.Content[0].(*mcp.TextContent)
			infoText := textContent.Text

			Expect(infoText).To(ContainSubstring("Environment:"))
			Expect(infoText).To(ContainSubstring("Home Directory:"))
			Expect(infoText).To(ContainSubstring("Current Directory:"))
			Expect(infoText).To(ContainSubstring("Shell: /bin/zsh"))
		})

		It("Should handle empty session directory configuration", func() {
			// Mock version service
			mockInfoService.EXPECT().GetVersion().Return("1.1.0").Times(1)

			// Set up environment
			_ = os.Setenv("BACKPLANE_URL", "https://api.backplane.example.com")
			viper.Set("govcloud", false)
			// Don't set session-dir, should use default

			// Execute the function
			result, _, err := mcptools.GetBackplaneInfo(context.Background(), &mcp.CallToolRequest{}, mcptools.BackplaneInfoInput{})

			// Verify results
			Expect(err).To(BeNil())
			Expect(result).ToNot(BeNil())

			textContent := result.Content[0].(*mcp.TextContent)
			infoText := textContent.Text

			Expect(infoText).To(ContainSubstring("Session Directory: backplane")) // default from info.BackplaneDefaultSessionDirectory
		})
	})

	Context("Input validation", func() {
		It("Should accept empty input struct", func() {
			// Mock version service
			mockInfoService.EXPECT().GetVersion().Return("1.0.0").Times(1)

			// Set up minimal environment
			_ = os.Setenv("BACKPLANE_URL", "https://api.backplane.example.com")
			viper.Set("govcloud", false)

			// Execute with empty input
			input := mcptools.BackplaneInfoInput{}
			result, _, err := mcptools.GetBackplaneInfo(context.Background(), &mcp.CallToolRequest{}, input)

			// Should work fine
			Expect(err).To(BeNil())
			Expect(result).ToNot(BeNil())
		})
	})
})
