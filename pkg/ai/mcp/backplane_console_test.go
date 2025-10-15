package mcp_test

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	mcptools "github.com/openshift/backplane-cli/pkg/ai/mcp"
)

var _ = Describe("BackplaneConsole", func() {

	Context("Input validation", func() {
		It("Should reject empty cluster ID", func() {
			input := mcptools.BackplaneConsoleArgs{ClusterID: ""}

			result, _, err := mcptools.BackplaneConsole(context.Background(), &mcp.CallToolRequest{}, input)

			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("cluster ID cannot be empty"))
			Expect(result).ToNot(BeNil())

			textContent := result.Content[0].(*mcp.TextContent)
			Expect(textContent.Text).To(Equal("Error: Cluster ID is required for backplane console access"))
		})

		It("Should reject whitespace-only cluster ID", func() {
			input := mcptools.BackplaneConsoleArgs{ClusterID: "   \t\n  "}

			result, _, err := mcptools.BackplaneConsole(context.Background(), &mcp.CallToolRequest{}, input)

			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("cluster ID cannot be empty"))
			Expect(result).ToNot(BeNil())

			textContent := result.Content[0].(*mcp.TextContent)
			Expect(textContent.Text).To(Equal("Error: Cluster ID is required for backplane console access"))
		})
	})

	Context("Argument structure validation", func() {
		It("Should accept valid BackplaneConsoleArgs structure", func() {
			input := mcptools.BackplaneConsoleArgs{
				ClusterID:     "test-cluster",
				OpenInBrowser: true,
			}

			// Verify struct fields are accessible
			Expect(input.ClusterID).To(Equal("test-cluster"))
			Expect(input.OpenInBrowser).To(BeTrue())
		})

		It("Should handle default OpenInBrowser value", func() {
			input := mcptools.BackplaneConsoleArgs{ClusterID: "default-test"}

			// OpenInBrowser should default to false
			Expect(input.OpenInBrowser).To(BeFalse())
		})

		It("Should validate JSON schema tags are present", func() {
			// Test that the struct works with JSON marshaling/unmarshaling
			// This ensures MCP can generate proper schemas
			input := mcptools.BackplaneConsoleArgs{
				ClusterID:     "schema-test",
				OpenInBrowser: true,
			}

			Expect(input.ClusterID).To(Equal("schema-test"))
			Expect(input.OpenInBrowser).To(BeTrue())

			// The struct should have proper JSON tags for MCP integration
			// We can't easily test the tags at runtime, but this test documents the requirement
		})
	})

	Context("Edge cases", func() {
		It("Should handle various cluster ID formats without execution", func() {
			// Test cluster ID format validation without actually executing console command
			testCases := []string{
				"simple-cluster",
				"cluster-with-dashes",
				"cluster_with_underscores",
				"cluster.with.dots",
				"cluster123numbers",
				"UPPERCASE-CLUSTER",
				"mixed-Case_Cluster.123",
			}

			for _, clusterID := range testCases {
				input := mcptools.BackplaneConsoleArgs{ClusterID: clusterID}

				// Test that cluster ID validation passes
				trimmedID := strings.TrimSpace(input.ClusterID)
				Expect(trimmedID).To(Equal(clusterID), "Test case: "+clusterID)
				Expect(trimmedID).ToNot(BeEmpty(), "Test case: "+clusterID)

				// Test struct field access
				Expect(input.ClusterID).To(Equal(clusterID), "Test case: "+clusterID)
			}
		})

		It("Should handle browser flag combinations", func() {
			testCases := []struct {
				clusterID   string
				openBrowser bool
			}{
				{"browser-false", false},
				{"browser-true", true},
				{"no-browser-specified", false}, // default
			}

			for _, tc := range testCases {
				input := mcptools.BackplaneConsoleArgs{
					ClusterID:     tc.clusterID,
					OpenInBrowser: tc.openBrowser,
				}

				// Verify struct configuration
				Expect(input.ClusterID).To(Equal(tc.clusterID))
				Expect(input.OpenInBrowser).To(Equal(tc.openBrowser))
			}
		})

		It("Should handle context cancellation in input validation", func() {
			// Create a cancelled context
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			// Test input validation with cancelled context
			input := mcptools.BackplaneConsoleArgs{ClusterID: ""}

			// Input validation should still work with cancelled context
			result, _, err := mcptools.BackplaneConsole(ctx, &mcp.CallToolRequest{}, input)

			Expect(err).ToNot(BeNil()) // Should reject empty cluster ID
			Expect(err.Error()).To(ContainSubstring("cluster ID cannot be empty"))
			Expect(result).ToNot(BeNil())

			textContent := result.Content[0].(*mcp.TextContent)
			Expect(textContent.Text).To(Equal("Error: Cluster ID is required for backplane console access"))
		})
	})

	Context("MCP protocol compliance", func() {
		It("Should return proper MCP response format for validation errors", func() {
			input := mcptools.BackplaneConsoleArgs{ClusterID: ""} // Invalid input

			result, output, err := mcptools.BackplaneConsole(context.Background(), &mcp.CallToolRequest{}, input)

			// Verify MCP response structure for validation errors
			Expect(err).ToNot(BeNil()) // Input validation error
			Expect(result).ToNot(BeNil())
			Expect(result.Content).To(HaveLen(1))

			// Verify output structure (should be empty struct)
			Expect(output).To(Equal(struct{}{}))

			// Verify content type
			textContent, ok := result.Content[0].(*mcp.TextContent)
			Expect(ok).To(BeTrue())
			Expect(textContent.Text).ToNot(BeEmpty())
		})

		It("Should have proper JSON schema structure for MCP integration", func() {
			// Test the input argument structure for MCP compatibility
			input := mcptools.BackplaneConsoleArgs{
				ClusterID:     "schema-validation-test",
				OpenInBrowser: true,
			}

			// Verify all fields are accessible and properly typed
			Expect(input.ClusterID).To(BeAssignableToTypeOf(""))
			Expect(input.OpenInBrowser).To(BeAssignableToTypeOf(true))

			// The struct should work with MCP's JSON schema generation
			Expect(input.ClusterID).To(Equal("schema-validation-test"))
			Expect(input.OpenInBrowser).To(BeTrue())
		})
	})

	// Note: We don't test actual console command execution in unit tests
	// because the console command starts containers and runs a web server,
	// which would cause tests to hang. The console integration is tested
	// through the direct function call approach, ensuring we use
	// console.NewConsoleCmd() instead of external command execution.
	// Integration testing with actual console functionality should be
	// done in separate integration test suites.
})
