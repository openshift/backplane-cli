package mcp

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type BackplaneClusterResourceArgs struct {
	Action        string `json:"action" jsonschema:"description:oc read action to perform (get, describe, logs, top, explain)"`
	ResourceType  string `json:"resourceType,omitempty" jsonschema:"description:kubernetes resource type (pod, service, deployment, configmap, secret, ingress, pvc, node, etc.)"`
	ResourceName  string `json:"resourceName,omitempty" jsonschema:"description:specific resource name (optional for get/describe actions)"`
	Namespace     string `json:"namespace,omitempty" jsonschema:"description:kubernetes namespace (use 'all' for all namespaces)"`
	OutputFormat  string `json:"outputFormat,omitempty" jsonschema:"description:output format (yaml, json, wide, name, custom-columns)"`
	LabelSelector string `json:"labelSelector,omitempty" jsonschema:"description:label selector filter (e.g., 'app=myapp,version=v1')"`
	FieldSelector string `json:"fieldSelector,omitempty" jsonschema:"description:field selector filter (e.g., 'status.phase=Running')"`
	AllNamespaces bool   `json:"allNamespaces,omitempty" jsonschema:"description:search across all namespaces"`
	Follow        bool   `json:"follow,omitempty" jsonschema:"description:follow logs in real-time (for logs action)"`
	Previous      bool   `json:"previous,omitempty" jsonschema:"description:get previous container logs (for logs action)"`
	Tail          int    `json:"tail,omitempty" jsonschema:"description:number of recent log lines to show (for logs action)"`
	Container     string `json:"container,omitempty" jsonschema:"description:container name for multi-container pods (for logs/exec actions)"`
	ShowLabels    bool   `json:"showLabels,omitempty" jsonschema:"description:show labels in output"`
	SortBy        string `json:"sortBy,omitempty" jsonschema:"description:sort output by field (e.g., '.metadata.creationTimestamp')"`
	Watch         bool   `json:"watch,omitempty" jsonschema:"description:watch for changes after listing/getting objects"`
	Raw           string `json:"raw,omitempty" jsonschema:"description:raw oc read arguments to pass directly"`
}

func BackplaneClusterResource(ctx context.Context, request *mcp.CallToolRequest, input BackplaneClusterResourceArgs) (*mcp.CallToolResult, any, error) {
	action := strings.TrimSpace(input.Action)
	if action == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Error: Action is required for cluster resource operations"},
			},
		}, nil, fmt.Errorf("action cannot be empty")
	}

	// Validate that only read actions are allowed
	allowedActions := []string{"get", "describe", "logs", "top", "explain"}
	actionLower := strings.ToLower(action)
	isAllowed := false
	for _, allowed := range allowedActions {
		if actionLower == allowed {
			isAllowed = true
			break
		}
	}
	if !isAllowed {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: Action '%s' is not allowed. Only read actions are supported: %s", action, strings.Join(allowedActions, ", "))},
			},
		}, nil, fmt.Errorf("unsupported action: %s", action)
	}

	// Build oc command arguments
	args := []string{}

	// If raw arguments are provided, use them directly
	if strings.TrimSpace(input.Raw) != "" {
		rawArgs := strings.Fields(strings.TrimSpace(input.Raw))
		args = append(args, rawArgs...)
	} else {
		// Build command based on action and parameters
		args = buildOcCommand(input)
	}

	// Execute the oc command
	cmd := exec.CommandContext(ctx, "oc", args...) //nolint:gosec

	// Capture both stdout and stderr
	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))

	if err != nil {
		errorMessage := fmt.Sprintf("Failed to execute oc %s", action)
		if strings.TrimSpace(input.ResourceType) != "" {
			errorMessage += fmt.Sprintf(" for resource type '%s'", strings.TrimSpace(input.ResourceType))
		}
		if strings.TrimSpace(input.ResourceName) != "" {
			errorMessage += fmt.Sprintf(" (resource: %s)", strings.TrimSpace(input.ResourceName))
		}
		if strings.TrimSpace(input.Namespace) != "" {
			errorMessage += fmt.Sprintf(" in namespace '%s'", strings.TrimSpace(input.Namespace))
		}
		errorMessage += fmt.Sprintf(". Error: %v", err)

		if outputStr != "" {
			errorMessage += fmt.Sprintf("\nCommand output: %s", outputStr)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: errorMessage},
			},
		}, nil, nil // Return nil error since we're handling it gracefully
	}

	// Build success message based on action
	var successMessage strings.Builder
	switch strings.ToLower(action) {
	case "get":
		if strings.TrimSpace(input.ResourceName) != "" {
			successMessage.WriteString(fmt.Sprintf("📋 Resource '%s/%s'", strings.TrimSpace(input.ResourceType), strings.TrimSpace(input.ResourceName)))
		} else {
			successMessage.WriteString(fmt.Sprintf("📋 Resources of type '%s'", strings.TrimSpace(input.ResourceType)))
		}
		if strings.TrimSpace(input.Namespace) != "" && strings.TrimSpace(input.Namespace) != "all" {
			successMessage.WriteString(fmt.Sprintf(" in namespace '%s'", strings.TrimSpace(input.Namespace)))
		} else if input.AllNamespaces || strings.TrimSpace(input.Namespace) == "all" {
			successMessage.WriteString(" across all namespaces")
		}
		successMessage.WriteString(":")

	case "describe":
		if strings.TrimSpace(input.ResourceName) != "" {
			successMessage.WriteString(fmt.Sprintf("📝 Detailed description of '%s/%s'", strings.TrimSpace(input.ResourceType), strings.TrimSpace(input.ResourceName)))
		} else {
			successMessage.WriteString(fmt.Sprintf("📝 Detailed description of '%s' resources", strings.TrimSpace(input.ResourceType)))
		}
		if strings.TrimSpace(input.Namespace) != "" && strings.TrimSpace(input.Namespace) != "all" {
			successMessage.WriteString(fmt.Sprintf(" in namespace '%s'", strings.TrimSpace(input.Namespace)))
		}
		successMessage.WriteString(":")

	case "logs":
		successMessage.WriteString(fmt.Sprintf("📄 Logs from '%s'", strings.TrimSpace(input.ResourceName)))
		if strings.TrimSpace(input.Container) != "" {
			successMessage.WriteString(fmt.Sprintf(" (container: %s)", strings.TrimSpace(input.Container)))
		}
		if strings.TrimSpace(input.Namespace) != "" {
			successMessage.WriteString(fmt.Sprintf(" in namespace '%s'", strings.TrimSpace(input.Namespace)))
		}
		if input.Follow {
			successMessage.WriteString(" (following)")
		}
		if input.Previous {
			successMessage.WriteString(" (previous container)")
		}
		successMessage.WriteString(":")

	// Removed delete case as write actions are not supported

	case "top":
		successMessage.WriteString(fmt.Sprintf("📊 Resource usage for '%s'", strings.TrimSpace(input.ResourceType)))
		if strings.TrimSpace(input.Namespace) != "" {
			successMessage.WriteString(fmt.Sprintf(" in namespace '%s'", strings.TrimSpace(input.Namespace)))
		}
		successMessage.WriteString(":")

	case "explain":
		successMessage.WriteString(fmt.Sprintf("📚 API documentation for '%s':", strings.TrimSpace(input.ResourceType)))

	// Removed apply case as write actions are not supported

	// Removed patch case as write actions are not supported

	// Removed scale case as write actions are not supported

	// Removed edit case as write actions are not supported

	default:
		successMessage.WriteString(fmt.Sprintf("✅ Executed oc %s", action))
		if strings.TrimSpace(input.ResourceType) != "" {
			successMessage.WriteString(fmt.Sprintf(" on '%s'", strings.TrimSpace(input.ResourceType)))
		}
		successMessage.WriteString(":")
	}

	if outputStr != "" {
		successMessage.WriteString(fmt.Sprintf("\n\n%s", outputStr))
	} else {
		successMessage.WriteString("\n\n(No output returned from command)")
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: successMessage.String()},
		},
	}, nil, nil
}

func buildOcCommand(input BackplaneClusterResourceArgs) []string {
	var args []string

	action := strings.ToLower(strings.TrimSpace(input.Action))
	args = append(args, action)

	// Handle different actions (only read actions allowed)
	switch action {
	case "get", "describe":
		if strings.TrimSpace(input.ResourceType) != "" {
			args = append(args, strings.TrimSpace(input.ResourceType))
		}
		if strings.TrimSpace(input.ResourceName) != "" {
			args = append(args, strings.TrimSpace(input.ResourceName))
		}

	case "logs":
		if strings.TrimSpace(input.ResourceName) == "" {
			// For logs, we need a pod name
			args = append(args, "pod")
		} else {
			args = append(args, strings.TrimSpace(input.ResourceName))
		}

	case "top":
		if strings.TrimSpace(input.ResourceType) != "" {
			args = append(args, strings.TrimSpace(input.ResourceType))
		} else {
			args = append(args, "pods") // Default to pods for top
		}

	case "explain":
		if strings.TrimSpace(input.ResourceType) != "" {
			args = append(args, strings.TrimSpace(input.ResourceType))
		}

	}

	// Add namespace
	if strings.TrimSpace(input.Namespace) != "" {
		if strings.TrimSpace(input.Namespace) == "all" || input.AllNamespaces {
			args = append(args, "--all-namespaces")
		} else {
			args = append(args, "--namespace", strings.TrimSpace(input.Namespace))
		}
	} else if input.AllNamespaces {
		args = append(args, "--all-namespaces")
	}

	// Add output format
	if strings.TrimSpace(input.OutputFormat) != "" {
		args = append(args, "-o", strings.TrimSpace(input.OutputFormat))
	}

	// Add selectors
	if strings.TrimSpace(input.LabelSelector) != "" {
		args = append(args, "-l", strings.TrimSpace(input.LabelSelector))
	}
	if strings.TrimSpace(input.FieldSelector) != "" {
		args = append(args, "--field-selector", strings.TrimSpace(input.FieldSelector))
	}

	// Add flags based on action (only read actions supported)
	switch action {
	case "logs":
		if input.Follow {
			args = append(args, "--follow")
		}
		if input.Previous {
			args = append(args, "--previous")
		}
		if input.Tail > 0 {
			args = append(args, "--tail", fmt.Sprintf("%d", input.Tail))
		}
		if strings.TrimSpace(input.Container) != "" {
			args = append(args, "-c", strings.TrimSpace(input.Container))
		}

	case "get", "describe":
		if input.ShowLabels {
			args = append(args, "--show-labels")
		}
		if strings.TrimSpace(input.SortBy) != "" {
			args = append(args, "--sort-by", strings.TrimSpace(input.SortBy))
		}
		if input.Watch {
			args = append(args, "--watch")
		}
	}

	return args
}
