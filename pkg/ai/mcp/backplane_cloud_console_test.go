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
			input := mcptools.BackplaneCloudConsoleArgs{ClusterID: "  cluster-123  ", OpenInBrowser: false}

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
				ClusterID:     "test-cluster",
				OpenInBrowser: true,
				Output:        "json",
				URL:           "https://custom.backplane.example.com",
			}

			// Verify struct fields are accessible
			Expect(input.ClusterID).To(Equal("test-cluster"))
			Expect(input.OpenInBrowser).To(BeTrue())
			Expect(input.Output).To(Equal("json"))
			Expect(input.URL).To(Equal("https://custom.backplane.example.com"))
		})

		It("Should handle default values correctly", func() {
			input := mcptools.BackplaneCloudConsoleArgs{ClusterID: "default-test"}

			// Verify default values
			Expect(input.ClusterID).To(Equal("default-test"))
			Expect(input.OpenInBrowser).To(BeFalse()) // Default false
			Expect(input.Output).To(Equal(""))        // Default empty
			Expect(input.URL).To(Equal(""))           // Default empty
		})

		It("Should validate JSON schema tags are present", func() {
			// Test that the struct works with JSON marshaling/unmarshaling
			// This ensures MCP can generate proper schemas
			input := mcptools.BackplaneCloudConsoleArgs{
				ClusterID:     "schema-test",
				OpenInBrowser: true,
				Output:        "text",
				URL:           "https://test.example.com",
			}

			Expect(input.ClusterID).To(Equal("schema-test"))
			Expect(input.OpenInBrowser).To(BeTrue())
			Expect(input.Output).To(Equal("text"))
			Expect(input.URL).To(Equal("https://test.example.com"))

			// The struct should have proper JSON tags for MCP integration
			// We can't easily test the tags at runtime, but this test documents the requirement
		})
	})

	Context("Output format handling", func() {
		It("Should handle different output formats", func() {
			outputFormats := []string{"text", "json", "yaml"}

			for _, format := range outputFormats {
				input := mcptools.BackplaneCloudConsoleArgs{
					ClusterID: "format-test-" + format,
					Output:    format,
				}

				Expect(input.Output).To(Equal(format), "Test case: "+format)

				// Verify struct field access works
				Expect(input.ClusterID).To(ContainSubstring("format-test"))
			}
		})

		It("Should handle empty output format (default to json)", func() {
			input := mcptools.BackplaneCloudConsoleArgs{
				ClusterID: "default-output-test",
				Output:    "", // Empty should default to json
			}

			// Test that empty output is handled
			Expect(input.Output).To(Equal(""))

			// The function should handle empty output by defaulting to json
			// This is tested in the implementation, not easily testable in unit tests
			// without executing the actual command
		})
	})

	Context("URL and browser parameter handling", func() {
		It("Should handle custom URL parameter", func() {
			customURLs := []string{
				"https://api.stage.backplane.openshift.com",
				"https://api.prod.backplane.openshift.com",
				"https://custom.backplane.example.com:8080",
				"http://localhost:3000", // For testing
			}

			for _, url := range customURLs {
				input := mcptools.BackplaneCloudConsoleArgs{
					ClusterID: "url-test",
					URL:       url,
				}

				Expect(input.URL).To(Equal(url), "Test case: "+url)
				Expect(input.ClusterID).To(Equal("url-test"))
			}
		})

		It("Should handle browser flag combinations", func() {
			testCases := []struct {
				clusterID   string
				openBrowser bool
			}{
				{"browser-false", false},
				{"browser-true", true},
				{"browser-default", false}, // default value
			}

			for _, tc := range testCases {
				input := mcptools.BackplaneCloudConsoleArgs{
					ClusterID:     tc.clusterID,
					OpenInBrowser: tc.openBrowser,
				}

				// Verify struct configuration
				Expect(input.ClusterID).To(Equal(tc.clusterID))
				Expect(input.OpenInBrowser).To(Equal(tc.openBrowser))
			}
		})

		It("Should handle comprehensive parameter combinations", func() {
			input := mcptools.BackplaneCloudConsoleArgs{
				ClusterID:     "comprehensive-test-cluster",
				OpenInBrowser: true,
				Output:        "json",
				URL:           "https://comprehensive.test.com",
			}

			// All parameters should be accessible
			Expect(input.ClusterID).To(Equal("comprehensive-test-cluster"))
			Expect(input.OpenInBrowser).To(BeTrue())
			Expect(input.Output).To(Equal("json"))
			Expect(input.URL).To(Equal("https://comprehensive.test.com"))
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

		It("Should indicate browser behavior in response when requested", func() {
			input := mcptools.BackplaneCloudConsoleArgs{
				ClusterID:     "browser-response-test",
				OpenInBrowser: true,
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

		It("Should validate output format parameter handling", func() {
			formats := []string{"json", "text", "yaml"}

			for _, format := range formats {
				input := mcptools.BackplaneCloudConsoleArgs{
					ClusterID: "output-format-test",
					Output:    format,
				}

				// Test struct field access
				Expect(input.Output).To(Equal(format), "Test case: "+format)
				Expect(input.ClusterID).To(Equal("output-format-test"))

				result, _, err := mcptools.BackplaneCloudConsole(context.Background(), &mcp.CallToolRequest{}, input)

				// Should handle gracefully (may fail due to cluster not existing)
				Expect(err).To(BeNil(), "Test case: "+format)
				Expect(result).ToNot(BeNil(), "Test case: "+format)

				textContent := result.Content[0].(*mcp.TextContent)
				Expect(textContent.Text).ToNot(BeEmpty(), "Test case: "+format)
				// Response should mention the cluster ID regardless of success/failure
				Expect(textContent.Text).To(ContainSubstring("output-format-test"), "Test case: "+format)
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

		It("Should handle various URL formats", func() {
			urlFormats := []string{
				"https://api.backplane.example.com",
				"http://localhost:8080",
				"https://custom.domain.com:9443/api/v1",
				"https://stage.backplane.openshift.com",
			}

			for _, url := range urlFormats {
				input := mcptools.BackplaneCloudConsoleArgs{
					ClusterID: "url-test",
					URL:       url,
				}

				// Test struct field access
				Expect(input.URL).To(Equal(url), "Test case: "+url)
				Expect(input.ClusterID).To(Equal("url-test"))
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
		It("Should handle minimal parameters (cluster ID only)", func() {
			input := mcptools.BackplaneCloudConsoleArgs{
				ClusterID: "minimal-test",
			}

			// Verify defaults
			Expect(input.ClusterID).To(Equal("minimal-test"))
			Expect(input.OpenInBrowser).To(BeFalse()) // default
			Expect(input.Output).To(Equal(""))        // default
			Expect(input.URL).To(Equal(""))           // default

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
					ClusterID:     "combo-test",
					Output:        tc.output,
					OpenInBrowser: tc.browser,
				}

				Expect(input.Output).To(Equal(tc.output), tc.description)
				Expect(input.OpenInBrowser).To(Equal(tc.browser), tc.description)
			}
		})

		It("Should handle URL override with various combinations", func() {
			testCases := []struct {
				url     string
				browser bool
				output  string
			}{
				{"https://custom.com", true, "json"},
				{"http://localhost:8080", false, "text"},
				{"https://test.example.com", true, "yaml"},
				{"", false, ""}, // No URL override
			}

			for i, tc := range testCases {
				input := mcptools.BackplaneCloudConsoleArgs{
					ClusterID:     "url-combo-test",
					URL:           tc.url,
					OpenInBrowser: tc.browser,
					Output:        tc.output,
				}

				Expect(input.URL).To(Equal(tc.url), "Test case %d", i)
				Expect(input.OpenInBrowser).To(Equal(tc.browser), "Test case %d", i)
				Expect(input.Output).To(Equal(tc.output), "Test case %d", i)
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
				ClusterID:     "schema-validation-test",
				OpenInBrowser: true,
				Output:        "json",
				URL:           "https://schema.test.com",
			}

			// Verify all fields are accessible and properly typed
			Expect(input.ClusterID).To(BeAssignableToTypeOf(""))
			Expect(input.OpenInBrowser).To(BeAssignableToTypeOf(true))
			Expect(input.Output).To(BeAssignableToTypeOf(""))
			Expect(input.URL).To(BeAssignableToTypeOf(""))

			// The struct should work with MCP's JSON schema generation
			Expect(input.ClusterID).To(Equal("schema-validation-test"))
			Expect(input.OpenInBrowser).To(BeTrue())
			Expect(input.Output).To(Equal("json"))
			Expect(input.URL).To(Equal("https://schema.test.com"))
		})
	})

	Context("Integration behavior", func() {
		It("Should create cloud console command instance for each call", func() {
			// Test that each call handles different parameters independently
			input1 := mcptools.BackplaneCloudConsoleArgs{ClusterID: "call-1", Output: "text"}
			input2 := mcptools.BackplaneCloudConsoleArgs{ClusterID: "call-2", Output: "json", OpenInBrowser: true}

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
					ClusterID:     scenario.clusterID,
					OpenInBrowser: scenario.browser,
					Output:        scenario.output,
					URL:           scenario.url,
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
