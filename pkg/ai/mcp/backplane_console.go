package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/console"
)

type BackplaneConsoleArgs struct {
	ClusterID     string `json:"clusterId" jsonschema:"description:the cluster ID for backplane console"`
	OpenInBrowser bool   `json:"openInBrowser,omitempty" jsonschema:"description:whether to automatically open the console URL in browser"`
}

func BackplaneConsole(ctx context.Context, request *mcp.CallToolRequest, input BackplaneConsoleArgs) (*mcp.CallToolResult, struct{}, error) {
	clusterID := strings.TrimSpace(input.ClusterID)
	if clusterID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Error: Cluster ID is required for backplane console access"},
			},
		}, struct{}{}, fmt.Errorf("cluster ID cannot be empty")
	}

	// Create console command and configure it
	consoleCmd := console.NewConsoleCmd()

	// Set up command arguments
	args := []string{clusterID}
	consoleCmd.SetArgs(args)

	// Configure flags if needed
	if input.OpenInBrowser {
		err := consoleCmd.Flags().Set("browser", "true")
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Error setting browser flag: %v", err)},
				},
			}, struct{}{}, nil
		}
	}

	// Call the console command's RunE function directly
	err := consoleCmd.RunE(consoleCmd, args)

	if err != nil {
		errorMessage := fmt.Sprintf("Failed to access console for cluster '%s'. Error: %v", clusterID, err)

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: errorMessage},
			},
		}, struct{}{}, nil // Return nil error since we're handling it gracefully
	}

	// Build success message
	var successMessage strings.Builder
	successMessage.WriteString(fmt.Sprintf("Successfully accessed cluster console for '%s'", clusterID))

	if input.OpenInBrowser {
		successMessage.WriteString("\n🌐 Console opened in default browser")
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: successMessage.String()},
		},
	}, struct{}{}, nil
}
