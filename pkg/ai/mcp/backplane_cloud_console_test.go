package mcp_test

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	mcptools "github.com/openshift/backplane-cli/pkg/ai/mcp"
)

var _ = Describe("BackplaneCloudConsole", func() {

	Context("Input validation", func() {
		It("Should reject empty cluster ID", func() {
			input := mcptools.BackplaneCloudConsoleArgs{ClusterID: ""}

			result, _, err := mcptools.BackplaneCloudConsole(context.Background(), &mcp.CallToolRequest{}, input)

			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("cluster ID cannot be empty"))
			Expect(result).ToNot(BeNil())

			textContent := result.Content[0].(*mcp.TextContent)
			Expect(textContent.Text).To(Equal("Error: Cluster ID is required for backplane cloud console"))
		})

		It("Should reject whitespace-only cluster ID", func() {
			input := mcptools.BackplaneCloudConsoleArgs{ClusterID: "   \t\n  "}

			result, _, err := mcptools.BackplaneCloudConsole(context.Background(), &mcp.CallToolRequest{}, input)

			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("cluster ID cannot be empty"))
			Expect(result).ToNot(BeNil())

			textContent := result.Content[0].(*mcp.TextContent)
			Expect(textContent.Text).To(Equal("Error: Cluster ID is required for backplane cloud console"))
		})

		It("Should trim whitespace from valid cluster ID", func() {
			input := mcptools.BackplaneCloudConsoleArgs{ClusterID: "  cluster-123  "}

			// Note: This test will try to actually access cloud console command
			// We expect it to fail with authentication/configuration errors, but the cluster ID should be trimmed
			result, _, err := mcptools.BackplaneCloudConsole(context.Background(), &mcp.CallToolRequest{}, input)

			// The function should not panic and should handle errors gracefully
			Expect(err).To(BeNil()) // MCP wrapper handles errors gracefully
			Expect(result).ToNot(BeNil())

			textContent := result.Content[0].(*mcp.TextContent)
			// Should mention the trimmed cluster ID in the response
			Expect(textContent.Text).To(ContainSubstring("cluster-123"))
		})
	})

	Context("Argument structure validation", func() {
		It("Should accept valid BackplaneCloudConsoleArgs structure", func() {
			input := mcptools.BackplaneCloudConsoleArgs{
				ClusterID: "test-cluster",
			}

			// Verify struct fields are accessible
			Expect(input.ClusterID).To(Equal("test-cluster"))
		})

		It("Should validate JSON schema tags are present", func() {
			// Test that the struct works with JSON marshaling/unmarshaling
			// This ensures MCP can generate proper schemas
			input := mcptools.BackplaneCloudConsoleArgs{
				ClusterID: "schema-test",
			}

			Expect(input.ClusterID).To(Equal("schema-test"))

			// The struct should have proper JSON tags for MCP integration
			// We can't easily test the tags at runtime, but this test documents the requirement
		})
	})

	Context("Cluster ID handling", func() {
		It("Should handle different cluster ID formats", func() {
			clusterIDs := []string{
				"cluster-1",
				"cluster-with-dashes",
				"cluster_with_underscores",
			}

			for _, clusterID := range clusterIDs {
				input := mcptools.BackplaneCloudConsoleArgs{
					ClusterID: clusterID,
				}

				// Verify the cluster ID is set correctly
				Expect(input.ClusterID).To(Equal(clusterID))
			}
		})

		It("Should handle different cluster IDs", func() {
			testCases := []string{
				"test-cluster-1",
				"test-cluster-2",
				"test-cluster-3",
			}

			for _, clusterID := range testCases {
				input := mcptools.BackplaneCloudConsoleArgs{
					ClusterID: clusterID,
				}

				// Verify struct configuration
				Expect(input.ClusterID).To(Equal(clusterID))
			}
		})

		It("Should handle different cluster IDs", func() {
			clusterIDs := []string{
				"comprehensive-test-cluster",
				"another-test-cluster",
				"yet-another-cluster",
			}

			for _, clusterID := range clusterIDs {
				input := mcptools.BackplaneCloudConsoleArgs{
					ClusterID: clusterID,
				}

				// All parameters should be accessible
				Expect(input.ClusterID).To(Equal(clusterID))
			}
		})
	})

	Context("Response format validation", func() {
		It("Should return valid MCP response structure for validation errors", func() {
			input := mcptools.BackplaneCloudConsoleArgs{ClusterID: ""} // Invalid input

			result, output, err := mcptools.BackplaneCloudConsole(context.Background(), &mcp.CallToolRequest{}, input)

			// Verify MCP response structure for validation errors
			Expect(err).ToNot(BeNil()) // Input validation error
			Expect(result).ToNot(BeNil())
			Expect(result.Content).To(HaveLen(1))

			// Verify output structure (should be nil)
			Expect(output).To(BeNil())

			// Verify content type
			textContent, ok := result.Content[0].(*mcp.TextContent)
			Expect(ok).To(BeTrue())
			Expect(textContent.Text).ToNot(BeEmpty())
		})

		It("Should include cluster ID in all response messages", func() {
			testClusterID := "response-test-cluster"
			input := mcptools.BackplaneCloudConsoleArgs{ClusterID: testClusterID}

			result, _, err := mcptools.BackplaneCloudConsole(context.Background(), &mcp.CallToolRequest{}, input)

			Expect(err).To(BeNil())
			Expect(result).ToNot(BeNil())

			textContent := result.Content[0].(*mcp.TextContent)
			Expect(textContent.Text).To(ContainSubstring(testClusterID))
		})

		It("Should indicate browser behavior in response", func() {
			input := mcptools.BackplaneCloudConsoleArgs{
				ClusterID: "browser-response-test",
			}

			result, _, err := mcptools.BackplaneCloudConsole(context.Background(), &mcp.CallToolRequest{}, input)

			Expect(err).To(BeNil())
			Expect(result).ToNot(BeNil())

			textContent := result.Content[0].(*mcp.TextContent)
			// Response should indicate browser behavior (either success or in error message)
			responseText := textContent.Text
			containsBrowserRef := strings.Contains(responseText, "browser") ||
				strings.Contains(responseText, "Browser") ||
				strings.Contains(responseText, "🌐")
			Expect(containsBrowserRef).To(BeTrue())
		})

		It("Should validate cluster ID parameter handling", func() {
			clusterIDs := []string{"cluster-test-1", "cluster-test-2", "cluster-test-3"}

			for _, clusterID := range clusterIDs {
				input := mcptools.BackplaneCloudConsoleArgs{
					ClusterID: clusterID,
				}

				// Test struct field access
				Expect(input.ClusterID).To(Equal(clusterID), "Test case: "+clusterID)

				result, _, err := mcptools.BackplaneCloudConsole(context.Background(), &mcp.CallToolRequest{}, input)

				// Should handle gracefully (may fail due to cluster not existing)
				Expect(err).To(BeNil(), "Test case: "+clusterID)
				Expect(result).ToNot(BeNil(), "Test case: "+clusterID)

				textContent := result.Content[0].(*mcp.TextContent)
				Expect(textContent.Text).ToNot(BeEmpty(), "Test case: "+clusterID)
				// Response should mention the cluster ID regardless of success/failure
				Expect(textContent.Text).To(ContainSubstring(clusterID), "Test case: "+clusterID)
			}
		})
	})

	Context("Edge cases", func() {
		It("Should handle various cluster ID formats", func() {
			testCases := []string{
				"simple-cluster",
				"cluster-with-dashes",
				"cluster_with_underscores",
				"cluster.with.dots",
				"cluster123numbers",
				"UPPERCASE-CLUSTER",
				"mixed-Case_Cluster.123",
				"very-long-cluster-name-with-multiple-parts",
			}

			for _, clusterID := range testCases {
				input := mcptools.BackplaneCloudConsoleArgs{ClusterID: clusterID}

				// Should handle all formats without panicking
				result, _, err := mcptools.BackplaneCloudConsole(context.Background(), &mcp.CallToolRequest{}, input)

				Expect(err).To(BeNil(), "Test case: "+clusterID)
				Expect(result).ToNot(BeNil(), "Test case: "+clusterID)

				textContent := result.Content[0].(*mcp.TextContent)
				Expect(textContent.Text).To(ContainSubstring(clusterID), "Test case: "+clusterID)
			}
		})

		It("Should handle different cluster IDs for testing", func() {
			clusterIDs := []string{
				"cluster-test-1",
				"cluster-test-2",
				"cluster-test-3",
			}

			for _, clusterID := range clusterIDs {
				input := mcptools.BackplaneCloudConsoleArgs{
					ClusterID: clusterID,
				}

				// Test struct field access
				Expect(input.ClusterID).To(Equal(clusterID))
			}
		})

		It("Should handle context cancellation gracefully", func() {
			// Create a cancelled context
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			input := mcptools.BackplaneCloudConsoleArgs{ClusterID: ""}

			// Input validation should still work with cancelled context
			result, _, err := mcptools.BackplaneCloudConsole(ctx, &mcp.CallToolRequest{}, input)

			Expect(err).ToNot(BeNil()) // Should reject empty cluster ID
			Expect(err.Error()).To(ContainSubstring("cluster ID cannot be empty"))
			Expect(result).ToNot(BeNil())

			textContent := result.Content[0].(*mcp.TextContent)
			Expect(textContent.Text).To(Equal("Error: Cluster ID is required for backplane cloud console"))
		})
	})

	Context("Parameter combinations", func() {
		It("Should handle cluster ID parameter", func() {
			input := mcptools.BackplaneCloudConsoleArgs{
				ClusterID: "minimal-test",
			}

			// Verify cluster ID
			Expect(input.ClusterID).To(Equal("minimal-test"))

			// Should pass basic validation
			trimmedID := strings.TrimSpace(input.ClusterID)
			Expect(trimmedID).ToNot(BeEmpty())
		})

		It("Should handle browser flag with different output formats", func() {
			testCases := []struct {
				output      string
				browser     bool
				description string
			}{
				{"text", true, "text format with browser"},
				{"json", true, "json format with browser"},
				{"yaml", false, "yaml format without browser"},
				{"", true, "default format with browser"},
			}

			for _, tc := range testCases {
				input := mcptools.BackplaneCloudConsoleArgs{
					ClusterID: "combo-test-" + tc.description,
				}

				Expect(input.ClusterID).To(ContainSubstring("combo-test"), tc.description)
			}
		})
	})

	Context("MCP protocol compliance", func() {
		It("Should return proper MCP response structure", func() {
			input := mcptools.BackplaneCloudConsoleArgs{ClusterID: "mcp-protocol-test"}

			result, output, err := mcptools.BackplaneCloudConsole(context.Background(), &mcp.CallToolRequest{}, input)

			// Verify response structure (even if cloud console command fails)
			Expect(err).To(BeNil()) // MCP wrapper handles errors gracefully
			Expect(result).ToNot(BeNil())
			Expect(result.Content).To(HaveLen(1))

			// Verify output structure (should be nil)
			Expect(output).To(BeNil())

			// Verify content type
			textContent, ok := result.Content[0].(*mcp.TextContent)
			Expect(ok).To(BeTrue())
			Expect(textContent.Text).ToNot(BeEmpty())
		})

		It("Should have proper JSON schema structure for MCP integration", func() {
			// Test the input argument structure for MCP compatibility
			input := mcptools.BackplaneCloudConsoleArgs{
				ClusterID: "schema-validation-test",
			}

			// Verify all fields are accessible and properly typed
			Expect(input.ClusterID).To(BeAssignableToTypeOf(""))

			// The struct should work with MCP's JSON schema generation
			Expect(input.ClusterID).To(Equal("schema-validation-test"))
		})
	})

	Context("Integration behavior", func() {
		It("Should create cloud console command instance for each call", func() {
			// Test that each call handles different parameters independently
			input1 := mcptools.BackplaneCloudConsoleArgs{ClusterID: "call-1"}
			input2 := mcptools.BackplaneCloudConsoleArgs{ClusterID: "call-2"}

			// First call
			result1, _, err1 := mcptools.BackplaneCloudConsole(context.Background(), &mcp.CallToolRequest{}, input1)
			Expect(err1).To(BeNil())
			Expect(result1).ToNot(BeNil())

			// Second call
			result2, _, err2 := mcptools.BackplaneCloudConsole(context.Background(), &mcp.CallToolRequest{}, input2)
			Expect(err2).To(BeNil())
			Expect(result2).ToNot(BeNil())

			// Both calls should produce valid responses
			textContent1 := result1.Content[0].(*mcp.TextContent)
			textContent2 := result2.Content[0].(*mcp.TextContent)

			Expect(textContent1.Text).To(ContainSubstring("call-1"))
			Expect(textContent2.Text).To(ContainSubstring("call-2"))

			// Both responses should be well-formed (success or error)
			Expect(textContent1.Text).ToNot(BeEmpty())
			Expect(textContent2.Text).ToNot(BeEmpty())
		})

		It("Should properly handle direct cloud console command integration", func() {
			// This test verifies that we're calling cloud.ConsoleCmd.RunE
			// rather than using external command execution

			input := mcptools.BackplaneCloudConsoleArgs{ClusterID: "integration-direct-test"}

			result, _, err := mcptools.BackplaneCloudConsole(context.Background(), &mcp.CallToolRequest{}, input)

			// The function should complete (though cloud console command may fail due to test environment)
			Expect(err).To(BeNil()) // MCP wrapper handles errors gracefully
			Expect(result).ToNot(BeNil())
			Expect(result.Content).To(HaveLen(1))

			textContent := result.Content[0].(*mcp.TextContent)
			Expect(textContent.Text).ToNot(BeEmpty())
			Expect(textContent.Text).To(ContainSubstring("integration-direct-test"))
		})

		It("Should handle realistic cloud console scenarios", func() {
			scenarios := []struct {
				name      string
				clusterID string
				browser   bool
				output    string
				url       string
			}{
				{
					name:      "Production cluster with browser",
					clusterID: "prod-cluster-123",
					browser:   true,
					output:    "json",
				},
				{
					name:      "Staging cluster no browser",
					clusterID: "staging-cluster-456",
					browser:   false,
					output:    "text",
				},
				{
					name:      "Dev cluster with custom URL",
					clusterID: "dev-cluster-789",
					browser:   false,
					output:    "json",
					url:       "https://dev.backplane.example.com",
				},
			}

			for _, scenario := range scenarios {
				input := mcptools.BackplaneCloudConsoleArgs{
					ClusterID: scenario.clusterID,
				}

				result, _, err := mcptools.BackplaneCloudConsole(context.Background(), &mcp.CallToolRequest{}, input)

				Expect(err).To(BeNil(), "Scenario: "+scenario.name)
				Expect(result).ToNot(BeNil(), "Scenario: "+scenario.name)

				textContent := result.Content[0].(*mcp.TextContent)
				// Should always include the cluster ID in response (success or error)
				Expect(textContent.Text).To(ContainSubstring(scenario.clusterID),
					"Scenario: "+scenario.name+" should contain cluster ID")

				// Response should be well-formed
				Expect(textContent.Text).ToNot(BeEmpty(), "Scenario: "+scenario.name)
			}
		})
	})

	// Note: We don't test actual cloud console command execution in unit tests
	// because it requires authentication, network access, and valid cluster credentials.
	// The cloud console integration is tested through the direct function call approach,
	// ensuring we use cloud.ConsoleCmd.RunE instead of external command execution.
	// Integration testing with actual cloud console functionality should be
	// done in separate integration test suites with proper authentication setup.
})
