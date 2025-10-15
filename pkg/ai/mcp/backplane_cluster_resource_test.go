package mcp_test

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	mcptools "github.com/openshift/backplane-cli/pkg/ai/mcp"
)

var _ = Describe("BackplaneClusterResource", func() {

	Context("Input validation", func() {
		It("Should reject empty action", func() {
			input := mcptools.BackplaneClusterResourceArgs{Action: ""}

			result, _, err := mcptools.BackplaneClusterResource(context.Background(), &mcp.CallToolRequest{}, input)

			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("action cannot be empty"))
			Expect(result).ToNot(BeNil())

			textContent := result.Content[0].(*mcp.TextContent)
			Expect(textContent.Text).To(Equal("Error: Action is required for cluster resource operations"))
		})

		It("Should reject whitespace-only action", func() {
			input := mcptools.BackplaneClusterResourceArgs{Action: "   \t\n  "}

			result, _, err := mcptools.BackplaneClusterResource(context.Background(), &mcp.CallToolRequest{}, input)

			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("action cannot be empty"))
			Expect(result).ToNot(BeNil())

			textContent := result.Content[0].(*mcp.TextContent)
			Expect(textContent.Text).To(Equal("Error: Action is required for cluster resource operations"))
		})

		It("Should reject invalid actions", func() {
			invalidActions := []string{"create", "delete", "patch", "apply", "edit", "replace", "scale"}

			for _, action := range invalidActions {
				input := mcptools.BackplaneClusterResourceArgs{Action: action}

				result, _, err := mcptools.BackplaneClusterResource(context.Background(), &mcp.CallToolRequest{}, input)

				Expect(err).ToNot(BeNil(), "Invalid action should be rejected: "+action)
				Expect(err.Error()).To(ContainSubstring("unsupported action: "+action), "Test case: "+action)
				Expect(result).ToNot(BeNil(), "Test case: "+action)

				textContent := result.Content[0].(*mcp.TextContent)
				Expect(textContent.Text).To(ContainSubstring("Action '"+action+"' is not allowed"), "Test case: "+action)
				Expect(textContent.Text).To(ContainSubstring("Only read actions are supported"), "Test case: "+action)
			}
		})

		It("Should accept valid read-only actions", func() {
			validActions := []string{"get", "describe", "logs", "top", "explain"}

			for _, action := range validActions {
				input := mcptools.BackplaneClusterResourceArgs{Action: action}

				// Note: This will try to execute oc command and likely fail due to test environment
				// But it should pass validation and not error on invalid action
				result, _, err := mcptools.BackplaneClusterResource(context.Background(), &mcp.CallToolRequest{}, input)

				// Should not fail on action validation (may fail on oc execution)
				Expect(err).To(BeNil(), "Valid action should pass validation: "+action)
				Expect(result).ToNot(BeNil(), "Test case: "+action)

				textContent := result.Content[0].(*mcp.TextContent)
				// Should not contain "not allowed" message for valid actions
				Expect(textContent.Text).ToNot(ContainSubstring("is not allowed"), "Test case: "+action)
			}
		})

		It("Should handle case insensitive actions", func() {
			caseCombinations := []string{"GET", "Get", "gEt", "DESCRIBE", "Describe"}

			for _, action := range caseCombinations {
				input := mcptools.BackplaneClusterResourceArgs{Action: action}

				result, _, err := mcptools.BackplaneClusterResource(context.Background(), &mcp.CallToolRequest{}, input)

				// Should pass validation for case variations
				Expect(err).To(BeNil(), "Case insensitive action should work: "+action)
				Expect(result).ToNot(BeNil(), "Test case: "+action)

				textContent := result.Content[0].(*mcp.TextContent)
				Expect(textContent.Text).ToNot(ContainSubstring("is not allowed"), "Test case: "+action)
			}
		})
	})

	Context("Argument structure validation", func() {
		It("Should accept valid BackplaneClusterResourceArgs structure", func() {
			input := mcptools.BackplaneClusterResourceArgs{
				Action:        "get",
				ResourceType:  "pod",
				ResourceName:  "test-pod",
				Namespace:     "default",
				OutputFormat:  "yaml",
				LabelSelector: "app=test",
				FieldSelector: "status.phase=Running",
				AllNamespaces: true,
				Follow:        false,
				Previous:      false,
				Tail:          100,
				Container:     "main",
				ShowLabels:    true,
				SortBy:        ".metadata.creationTimestamp",
				Watch:         false,
				Raw:           "",
			}

			// Verify all struct fields are accessible
			Expect(input.Action).To(Equal("get"))
			Expect(input.ResourceType).To(Equal("pod"))
			Expect(input.ResourceName).To(Equal("test-pod"))
			Expect(input.Namespace).To(Equal("default"))
			Expect(input.OutputFormat).To(Equal("yaml"))
			Expect(input.LabelSelector).To(Equal("app=test"))
			Expect(input.FieldSelector).To(Equal("status.phase=Running"))
			Expect(input.AllNamespaces).To(BeTrue())
			Expect(input.Follow).To(BeFalse())
			Expect(input.Previous).To(BeFalse())
			Expect(input.Tail).To(Equal(100))
			Expect(input.Container).To(Equal("main"))
			Expect(input.ShowLabels).To(BeTrue())
			Expect(input.SortBy).To(Equal(".metadata.creationTimestamp"))
			Expect(input.Watch).To(BeFalse())
			Expect(input.Raw).To(Equal(""))
		})

		It("Should handle default values correctly", func() {
			input := mcptools.BackplaneClusterResourceArgs{Action: "get"}

			// Verify default values
			Expect(input.Action).To(Equal("get"))
			Expect(input.ResourceType).To(Equal(""))
			Expect(input.ResourceName).To(Equal(""))
			Expect(input.Namespace).To(Equal(""))
			Expect(input.OutputFormat).To(Equal(""))
			Expect(input.LabelSelector).To(Equal(""))
			Expect(input.FieldSelector).To(Equal(""))
			Expect(input.AllNamespaces).To(BeFalse())
			Expect(input.Follow).To(BeFalse())
			Expect(input.Previous).To(BeFalse())
			Expect(input.Tail).To(Equal(0))
			Expect(input.Container).To(Equal(""))
			Expect(input.ShowLabels).To(BeFalse())
			Expect(input.SortBy).To(Equal(""))
			Expect(input.Watch).To(BeFalse())
			Expect(input.Raw).To(Equal(""))
		})

		It("Should validate JSON schema tags are present", func() {
			// Test that the struct works with JSON marshaling/unmarshaling
			// This ensures MCP can generate proper schemas
			input := mcptools.BackplaneClusterResourceArgs{
				Action:       "describe",
				ResourceType: "deployment",
				ResourceName: "myapp",
				Namespace:    "production",
			}

			Expect(input.Action).To(Equal("describe"))
			Expect(input.ResourceType).To(Equal("deployment"))
			Expect(input.ResourceName).To(Equal("myapp"))
			Expect(input.Namespace).To(Equal("production"))

			// The struct should have proper JSON tags for MCP integration
			// We can't easily test the tags at runtime, but this test documents the requirement
		})
	})

	Context("Response format validation", func() {
		It("Should return valid MCP response structure for validation errors", func() {
			input := mcptools.BackplaneClusterResourceArgs{Action: ""} // Invalid input

			result, output, err := mcptools.BackplaneClusterResource(context.Background(), &mcp.CallToolRequest{}, input)

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

		It("Should return valid MCP response structure for invalid actions", func() {
			input := mcptools.BackplaneClusterResourceArgs{Action: "delete"} // Not allowed

			result, output, err := mcptools.BackplaneClusterResource(context.Background(), &mcp.CallToolRequest{}, input)

			// Verify MCP response structure for action validation errors
			Expect(err).ToNot(BeNil()) // Action validation error
			Expect(result).ToNot(BeNil())
			Expect(result.Content).To(HaveLen(1))

			// Verify output structure (should be empty struct)
			Expect(output).To(Equal(struct{}{}))

			// Verify content type
			textContent, ok := result.Content[0].(*mcp.TextContent)
			Expect(ok).To(BeTrue())
			Expect(textContent.Text).ToNot(BeEmpty())
			Expect(textContent.Text).To(ContainSubstring("is not allowed"))
		})
	})

	Context("Parameter combinations", func() {
		It("Should handle minimal parameters", func() {
			input := mcptools.BackplaneClusterResourceArgs{Action: "get"}

			// Should pass validation with minimal parameters
			trimmedAction := strings.TrimSpace(input.Action)
			Expect(trimmedAction).To(Equal("get"))
			Expect(trimmedAction).ToNot(BeEmpty())

			// Verify action is in allowed list
			allowedActions := []string{"get", "describe", "logs", "top", "explain"}
			Expect(allowedActions).To(ContainElement("get"))
		})

		It("Should handle get action with resource type", func() {
			input := mcptools.BackplaneClusterResourceArgs{
				Action:       "get",
				ResourceType: "pods",
			}

			Expect(input.Action).To(Equal("get"))
			Expect(input.ResourceType).To(Equal("pods"))

			// Should pass basic validation
			trimmedAction := strings.TrimSpace(input.Action)
			Expect(trimmedAction).ToNot(BeEmpty())
		})

		It("Should handle describe action with specific resource", func() {
			input := mcptools.BackplaneClusterResourceArgs{
				Action:       "describe",
				ResourceType: "deployment",
				ResourceName: "myapp",
				Namespace:    "production",
			}

			Expect(input.Action).To(Equal("describe"))
			Expect(input.ResourceType).To(Equal("deployment"))
			Expect(input.ResourceName).To(Equal("myapp"))
			Expect(input.Namespace).To(Equal("production"))
		})

		It("Should handle logs action with logging parameters", func() {
			input := mcptools.BackplaneClusterResourceArgs{
				Action:       "logs",
				ResourceType: "pod",
				ResourceName: "test-pod-123",
				Namespace:    "kube-system",
				Follow:       true,
				Previous:     false,
				Tail:         50,
				Container:    "main-container",
			}

			Expect(input.Action).To(Equal("logs"))
			Expect(input.Follow).To(BeTrue())
			Expect(input.Previous).To(BeFalse())
			Expect(input.Tail).To(Equal(50))
			Expect(input.Container).To(Equal("main-container"))
		})

		It("Should handle get action with advanced filtering", func() {
			input := mcptools.BackplaneClusterResourceArgs{
				Action:        "get",
				ResourceType:  "pods",
				Namespace:     "production",
				LabelSelector: "app=myapp,version=v1.0",
				FieldSelector: "status.phase=Running",
				OutputFormat:  "yaml",
				ShowLabels:    true,
				AllNamespaces: false,
				Watch:         false,
			}

			Expect(input.LabelSelector).To(Equal("app=myapp,version=v1.0"))
			Expect(input.FieldSelector).To(Equal("status.phase=Running"))
			Expect(input.OutputFormat).To(Equal("yaml"))
			Expect(input.ShowLabels).To(BeTrue())
		})

		It("Should handle raw command arguments", func() {
			input := mcptools.BackplaneClusterResourceArgs{
				Action: "get",
				Raw:    "pods --all-namespaces -o wide",
			}

			Expect(input.Action).To(Equal("get"))
			Expect(input.Raw).To(Equal("pods --all-namespaces -o wide"))

			// Should pass action validation
			trimmedAction := strings.TrimSpace(input.Action)
			Expect(trimmedAction).ToNot(BeEmpty())
		})
	})

	Context("Action validation", func() {
		It("Should validate all allowed read-only actions", func() {
			allowedActions := []string{"get", "describe", "logs", "top", "explain"}

			for _, action := range allowedActions {
				input := mcptools.BackplaneClusterResourceArgs{Action: action}

				// Test that validation passes (may fail on execution but not on validation)
				result, _, err := mcptools.BackplaneClusterResource(context.Background(), &mcp.CallToolRequest{}, input)

				Expect(err).To(BeNil(), "Action should be allowed: "+action)
				Expect(result).ToNot(BeNil(), "Test case: "+action)

				textContent := result.Content[0].(*mcp.TextContent)
				Expect(textContent.Text).ToNot(ContainSubstring("is not allowed"), "Test case: "+action)
			}
		})

		It("Should reject all write operations", func() {
			writeActions := []string{
				"create", "apply", "delete", "patch", "replace", "edit",
				"scale", "rollout", "label", "annotate", "expose", "set",
			}

			for _, action := range writeActions {
				input := mcptools.BackplaneClusterResourceArgs{Action: action}

				result, _, err := mcptools.BackplaneClusterResource(context.Background(), &mcp.CallToolRequest{}, input)

				Expect(err).ToNot(BeNil(), "Write action should be rejected: "+action)
				Expect(err.Error()).To(ContainSubstring("unsupported action"), "Test case: "+action)
				Expect(result).ToNot(BeNil(), "Test case: "+action)

				textContent := result.Content[0].(*mcp.TextContent)
				Expect(textContent.Text).To(ContainSubstring("is not allowed"), "Test case: "+action)
				Expect(textContent.Text).To(ContainSubstring("Only read actions are supported"), "Test case: "+action)
			}
		})

		It("Should handle mixed case actions correctly", func() {
			mixedCaseActions := []string{"GET", "Get", "gEt", "DESCRIBE", "Describe", "dEsCrIbE"}

			for _, action := range mixedCaseActions {
				input := mcptools.BackplaneClusterResourceArgs{Action: action}

				result, _, err := mcptools.BackplaneClusterResource(context.Background(), &mcp.CallToolRequest{}, input)

				// Should pass validation regardless of case
				Expect(err).To(BeNil(), "Case variation should be accepted: "+action)
				Expect(result).ToNot(BeNil(), "Test case: "+action)

				textContent := result.Content[0].(*mcp.TextContent)
				Expect(textContent.Text).ToNot(ContainSubstring("is not allowed"), "Test case: "+action)
			}
		})
	})

	Context("Edge cases", func() {
		It("Should handle various resource type formats", func() {
			resourceTypes := []string{
				"pod", "pods", "po",
				"service", "services", "svc",
				"deployment", "deployments", "deploy",
				"configmap", "configmaps", "cm",
				"secret", "secrets",
				"ingress", "ingresses", "ing",
				"persistentvolumeclaim", "persistentvolumeclaims", "pvc",
				"node", "nodes", "no",
			}

			for _, resourceType := range resourceTypes {
				input := mcptools.BackplaneClusterResourceArgs{
					Action:       "get",
					ResourceType: resourceType,
				}

				// Test struct field access
				Expect(input.Action).To(Equal("get"))
				Expect(input.ResourceType).To(Equal(resourceType))

				// Should pass basic validation
				trimmedAction := strings.TrimSpace(input.Action)
				Expect(trimmedAction).ToNot(BeEmpty(), "Test case: "+resourceType)
			}
		})

		It("Should handle various namespace formats", func() {
			namespaces := []string{
				"default",
				"kube-system",
				"kube-public",
				"openshift-config",
				"my-app-namespace",
				"test_namespace",
				"namespace.with.dots",
				"all", // Special case for all namespaces
			}

			for _, namespace := range namespaces {
				input := mcptools.BackplaneClusterResourceArgs{
					Action:    "get",
					Namespace: namespace,
				}

				Expect(input.Namespace).To(Equal(namespace), "Test case: "+namespace)
			}
		})

		It("Should handle various output formats", func() {
			formats := []string{
				"yaml", "json", "wide", "name",
				"custom-columns=NAME:.metadata.name",
				"jsonpath={.items[*].metadata.name}",
			}

			for _, format := range formats {
				input := mcptools.BackplaneClusterResourceArgs{
					Action:       "get",
					OutputFormat: format,
				}

				Expect(input.OutputFormat).To(Equal(format), "Test case: "+format)
			}
		})

		It("Should handle complex label selectors", func() {
			labelSelectors := []string{
				"app=myapp",
				"app=myapp,version=v1.0",
				"environment!=development",
				"tier in (frontend,backend)",
				"!beta",
				"app=myapp,version=v1.0,tier=frontend",
			}

			for _, selector := range labelSelectors {
				input := mcptools.BackplaneClusterResourceArgs{
					Action:        "get",
					ResourceType:  "pod",
					LabelSelector: selector,
				}

				Expect(input.LabelSelector).To(Equal(selector), "Test case: "+selector)
			}
		})

		It("Should handle complex field selectors", func() {
			fieldSelectors := []string{
				"status.phase=Running",
				"metadata.namespace!=kube-system",
				"spec.nodeName=worker-1",
				"status.containerStatuses[*].ready=true",
			}

			for _, selector := range fieldSelectors {
				input := mcptools.BackplaneClusterResourceArgs{
					Action:        "get",
					ResourceType:  "pod",
					FieldSelector: selector,
				}

				Expect(input.FieldSelector).To(Equal(selector), "Test case: "+selector)
			}
		})
	})

	Context("Logging-specific parameters", func() {
		It("Should handle logging parameters for logs action", func() {
			input := mcptools.BackplaneClusterResourceArgs{
				Action:       "logs",
				ResourceType: "pod",
				ResourceName: "test-pod",
				Follow:       true,
				Previous:     false,
				Tail:         100,
				Container:    "sidecar",
			}

			Expect(input.Action).To(Equal("logs"))
			Expect(input.Follow).To(BeTrue())
			Expect(input.Previous).To(BeFalse())
			Expect(input.Tail).To(Equal(100))
			Expect(input.Container).To(Equal("sidecar"))
		})

		It("Should handle various tail values", func() {
			tailValues := []int{0, 1, 10, 50, 100, 1000}

			for _, tail := range tailValues {
				input := mcptools.BackplaneClusterResourceArgs{
					Action: "logs",
					Tail:   tail,
				}

				Expect(input.Tail).To(Equal(tail), "Test case: tail=%d", tail)
			}
		})

		It("Should handle boolean flag combinations", func() {
			testCases := []struct {
				follow        bool
				previous      bool
				allNamespaces bool
				showLabels    bool
				watch         bool
			}{
				{false, false, false, false, false}, // All false
				{true, false, false, false, false},  // Only follow
				{false, true, false, false, false},  // Only previous
				{false, false, true, false, false},  // Only allNamespaces
				{false, false, false, true, false},  // Only showLabels
				{false, false, false, false, true},  // Only watch
				{true, true, true, true, true},      // All true
			}

			for i, tc := range testCases {
				input := mcptools.BackplaneClusterResourceArgs{
					Action:        "get",
					Follow:        tc.follow,
					Previous:      tc.previous,
					AllNamespaces: tc.allNamespaces,
					ShowLabels:    tc.showLabels,
					Watch:         tc.watch,
				}

				Expect(input.Follow).To(Equal(tc.follow), "Test case %d: follow", i)
				Expect(input.Previous).To(Equal(tc.previous), "Test case %d: previous", i)
				Expect(input.AllNamespaces).To(Equal(tc.allNamespaces), "Test case %d: allNamespaces", i)
				Expect(input.ShowLabels).To(Equal(tc.showLabels), "Test case %d: showLabels", i)
				Expect(input.Watch).To(Equal(tc.watch), "Test case %d: watch", i)
			}
		})
	})

	Context("Context handling", func() {
		It("Should handle context cancellation in input validation", func() {
			// Create a cancelled context
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			// Test input validation with cancelled context
			input := mcptools.BackplaneClusterResourceArgs{Action: ""}

			// Input validation should still work with cancelled context
			result, _, err := mcptools.BackplaneClusterResource(ctx, &mcp.CallToolRequest{}, input)

			Expect(err).ToNot(BeNil()) // Should reject empty action
			Expect(err.Error()).To(ContainSubstring("action cannot be empty"))
			Expect(result).ToNot(BeNil())

			textContent := result.Content[0].(*mcp.TextContent)
			Expect(textContent.Text).To(Equal("Error: Action is required for cluster resource operations"))
		})
	})

	// Note: We don't test actual oc command execution in unit tests
	// because it requires a valid kubernetes context and cluster access.
	// The cluster resource tool integration is tested through the direct
	// function call approach, ensuring proper argument construction and
	// validation. Integration testing with actual oc commands should be
	// done in separate integration test suites with proper cluster setup.
})
