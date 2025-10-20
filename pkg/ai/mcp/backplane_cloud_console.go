package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/cloud"
)

type BackplaneCloudConsoleArgs struct {
	ClusterID string `json:"clusterId" jsonschema:"description:the cluster ID for backplane cloud console"`
}

func BackplaneCloudConsole(ctx context.Context, request *mcp.CallToolRequest, input BackplaneCloudConsoleArgs) (*mcp.CallToolResult, any, error) {
	clusterID := strings.TrimSpace(input.ClusterID)
	if clusterID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Error: Cluster ID is required for backplane cloud console"},
			},
		}, nil, fmt.Errorf("cluster ID cannot be empty")
	}

	// Create cloud console command and configure it
	consoleCmd := cloud.ConsoleCmd

	// Set up command arguments
	args := []string{clusterID}
	consoleCmd.SetArgs(args)

	// Always open in browser when using MCP
	err := consoleCmd.Flags().Set("browser", "true")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error setting browser flag: %v", err)},
			},
		}, nil, nil
	}

	// Set output format to json for better parsing
	err = consoleCmd.Flags().Set("output", "json")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error setting output flag: %v", err)},
			},
		}, nil, nil
	}

	// Run the cloud console command in a background goroutine to avoid blocking
	errChan := make(chan error, 1)
	go func() {
		errChan <- consoleCmd.RunE(consoleCmd, args)
	}()

	// Wait briefly to see if there's an immediate error (e.g., login required, invalid cluster)
	select {
	case err := <-errChan:
		// Command failed quickly - likely a configuration/validation error
		errorMessage := fmt.Sprintf("Failed to get cloud console for cluster '%s'. Error: %v", clusterID, err)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: errorMessage},
			},
		}, nil, nil
	case <-time.After(3 * time.Second):
		// No immediate error - cloud console is starting up successfully
		// The goroutine continues running in the background
	}

	// Build success message
	var successMessage strings.Builder
	successMessage.WriteString(fmt.Sprintf("✅ Cloud console access retrieved for cluster '%s'\n\n", clusterID))
	successMessage.WriteString("🌐 Cloud console will open in your default browser when ready\n")
	successMessage.WriteString("\n⚠️  Note: The cloud console command is running in the background")

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: successMessage.String()},
		},
	}, nil, nil
}
