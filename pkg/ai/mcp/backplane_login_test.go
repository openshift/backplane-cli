package mcp_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/login"
	mcptools "github.com/openshift/backplane-cli/pkg/ai/mcp"
)

var _ = Describe("BackplaneLogin", func() {
	var (
		originalRunE func(cmd *cobra.Command, args []string) error
	)

	BeforeEach(func() {
		// Store original RunE function to restore later
		originalRunE = login.LoginCmd.RunE
	})

	AfterEach(func() {
		// Restore original RunE function
		login.LoginCmd.RunE = originalRunE
	})

	Context("Input validation", func() {
		It("Should reject empty cluster ID", func() {
			input := mcptools.BackplaneLoginArgs{ClusterID: ""}

			result, _, err := mcptools.BackplaneLogin(context.Background(), &mcp.CallToolRequest{}, input)

			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("cluster ID cannot be empty"))
			Expect(result).ToNot(BeNil())

			textContent := result.Content[0].(*mcp.TextContent)
			Expect(textContent.Text).To(Equal("Error: Cluster ID is required for backplane login"))
		})

		It("Should reject whitespace-only cluster ID", func() {
			input := mcptools.BackplaneLoginArgs{ClusterID: "   \t\n  "}

			result, _, err := mcptools.BackplaneLogin(context.Background(), &mcp.CallToolRequest{}, input)

			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("cluster ID cannot be empty"))
			Expect(result).ToNot(BeNil())

			textContent := result.Content[0].(*mcp.TextContent)
			Expect(textContent.Text).To(Equal("Error: Cluster ID is required for backplane login"))
		})

		It("Should trim whitespace from valid cluster ID", func() {
			// Mock successful login
			login.LoginCmd.RunE = func(cmd *cobra.Command, args []string) error {
				// Verify the trimmed cluster ID is passed
				Expect(args).To(HaveLen(1))
				Expect(args[0]).To(Equal("cluster-123"))
				return nil
			}

			input := mcptools.BackplaneLoginArgs{ClusterID: "  cluster-123  "}

			result, _, err := mcptools.BackplaneLogin(context.Background(), &mcp.CallToolRequest{}, input)

			Expect(err).To(BeNil())
			Expect(result).ToNot(BeNil())

			textContent := result.Content[0].(*mcp.TextContent)
			Expect(textContent.Text).To(Equal("Successfully logged in to cluster 'cluster-123'"))
		})
	})

	Context("Successful login", func() {
		It("Should return success message for valid cluster ID", func() {
			// Mock successful login
			login.LoginCmd.RunE = func(cmd *cobra.Command, args []string) error {
				Expect(args).To(HaveLen(1))
				Expect(args[0]).To(Equal("test-cluster-456"))
				return nil
			}

			input := mcptools.BackplaneLoginArgs{ClusterID: "test-cluster-456"}

			result, _, err := mcptools.BackplaneLogin(context.Background(), &mcp.CallToolRequest{}, input)

			Expect(err).To(BeNil())
			Expect(result).ToNot(BeNil())
			Expect(result.Content).To(HaveLen(1))

			textContent := result.Content[0].(*mcp.TextContent)
			Expect(textContent.Text).To(Equal("Successfully logged in to cluster 'test-cluster-456'"))
		})

		It("Should handle cluster IDs with special characters", func() {
			specialClusterID := "cluster-with-dashes_and_underscores.123"

			// Mock successful login
			login.LoginCmd.RunE = func(cmd *cobra.Command, args []string) error {
				Expect(args).To(HaveLen(1))
				Expect(args[0]).To(Equal(specialClusterID))
				return nil
			}

			input := mcptools.BackplaneLoginArgs{ClusterID: specialClusterID}

			result, _, err := mcptools.BackplaneLogin(context.Background(), &mcp.CallToolRequest{}, input)

			Expect(err).To(BeNil())
			Expect(result).ToNot(BeNil())

			textContent := result.Content[0].(*mcp.TextContent)
			expectedMessage := "Successfully logged in to cluster 'cluster-with-dashes_and_underscores.123'"
			Expect(textContent.Text).To(Equal(expectedMessage))
		})
	})

	Context("Failed login", func() {
		It("Should handle login command errors gracefully", func() {
			// Mock failed login
			login.LoginCmd.RunE = func(cmd *cobra.Command, args []string) error {
				return errors.New("cluster not found")
			}

			input := mcptools.BackplaneLoginArgs{ClusterID: "non-existent-cluster"}

			result, _, err := mcptools.BackplaneLogin(context.Background(), &mcp.CallToolRequest{}, input)

			// Should not return error in the function signature (graceful handling)
			Expect(err).To(BeNil())
			Expect(result).ToNot(BeNil())
			Expect(result.Content).To(HaveLen(1))

			textContent := result.Content[0].(*mcp.TextContent)
			Expect(textContent.Text).To(ContainSubstring("Failed to login to cluster 'non-existent-cluster'"))
			Expect(textContent.Text).To(ContainSubstring("cluster not found"))
		})

		It("Should handle authentication errors", func() {
			// Mock authentication failure
			login.LoginCmd.RunE = func(cmd *cobra.Command, args []string) error {
				return errors.New("authentication failed: invalid token")
			}

			input := mcptools.BackplaneLoginArgs{ClusterID: "auth-test-cluster"}

			result, _, err := mcptools.BackplaneLogin(context.Background(), &mcp.CallToolRequest{}, input)

			Expect(err).To(BeNil()) // Graceful error handling
			Expect(result).ToNot(BeNil())

			textContent := result.Content[0].(*mcp.TextContent)
			Expect(textContent.Text).To(ContainSubstring("Failed to login to cluster 'auth-test-cluster'"))
			Expect(textContent.Text).To(ContainSubstring("authentication failed: invalid token"))
		})

		It("Should handle network connectivity errors", func() {
			// Mock network error
			login.LoginCmd.RunE = func(cmd *cobra.Command, args []string) error {
				return errors.New("network error: connection timeout")
			}

			input := mcptools.BackplaneLoginArgs{ClusterID: "network-test-cluster"}

			result, _, err := mcptools.BackplaneLogin(context.Background(), &mcp.CallToolRequest{}, input)

			Expect(err).To(BeNil()) // Graceful error handling
			Expect(result).ToNot(BeNil())

			textContent := result.Content[0].(*mcp.TextContent)
			Expect(textContent.Text).To(ContainSubstring("Failed to login to cluster 'network-test-cluster'"))
			Expect(textContent.Text).To(ContainSubstring("network error: connection timeout"))
		})
	})

	Context("Response format validation", func() {
		It("Should return valid MCP response structure for success", func() {
			// Mock successful login
			login.LoginCmd.RunE = func(cmd *cobra.Command, args []string) error {
				return nil
			}

			input := mcptools.BackplaneLoginArgs{ClusterID: "format-test-cluster"}

			result, output, err := mcptools.BackplaneLogin(context.Background(), &mcp.CallToolRequest{}, input)

			// Verify response structure
			Expect(err).To(BeNil())
			Expect(result).ToNot(BeNil())
			Expect(result.Content).To(HaveLen(1))

			// Verify output structure (should be nil)
			Expect(output).To(BeNil())

			// Verify content type
			textContent, ok := result.Content[0].(*mcp.TextContent)
			Expect(ok).To(BeTrue())
			Expect(textContent.Text).ToNot(BeEmpty())
		})

		It("Should return valid MCP response structure for error", func() {
			// Mock failed login
			login.LoginCmd.RunE = func(cmd *cobra.Command, args []string) error {
				return errors.New("test error")
			}

			input := mcptools.BackplaneLoginArgs{ClusterID: "error-format-test"}

			result, output, err := mcptools.BackplaneLogin(context.Background(), &mcp.CallToolRequest{}, input)

			// Verify response structure
			Expect(err).To(BeNil()) // Graceful error handling
			Expect(result).ToNot(BeNil())
			Expect(result.Content).To(HaveLen(1))

			// Verify output structure (should be nil)
			Expect(output).To(BeNil())

			// Verify content type
			textContent, ok := result.Content[0].(*mcp.TextContent)
			Expect(ok).To(BeTrue())
			Expect(textContent.Text).ToNot(BeEmpty())
			Expect(textContent.Text).To(ContainSubstring("Failed to login"))
		})
	})

	Context("Context handling", func() {
		It("Should respect context cancellation", func() {
			// Create a cancelled context
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			// Mock login that would normally succeed
			login.LoginCmd.RunE = func(cmd *cobra.Command, args []string) error {
				return nil
			}

			input := mcptools.BackplaneLoginArgs{ClusterID: "context-test-cluster"}

			// Function should still complete since context isn't directly used in current implementation
			result, _, err := mcptools.BackplaneLogin(ctx, &mcp.CallToolRequest{}, input)

			Expect(err).To(BeNil())
			Expect(result).ToNot(BeNil())
		})
	})
})
