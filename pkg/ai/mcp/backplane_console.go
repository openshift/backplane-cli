package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/console"
)

type BackplaneConsoleArgs struct {
	ClusterID string `json:"clusterId" jsonschema:"description:the cluster ID for backplane console"`
}

func BackplaneConsole(ctx context.Context, request *mcp.CallToolRequest, input BackplaneConsoleArgs) (*mcp.CallToolResult, any, error) {
	clusterID := strings.TrimSpace(input.ClusterID)
	if clusterID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Error: Cluster ID is required for backplane console access"},
			},
		}, nil, fmt.Errorf("cluster ID cannot be empty")
	}

	// Create console command and configure it
	consoleCmd := console.NewConsoleCmd()

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

	// Run the console command in a background goroutine to avoid blocking
	// The console command blocks indefinitely waiting for Ctrl+C
	errChan := make(chan error, 1)
	go func() {
		errChan <- consoleCmd.RunE(consoleCmd, args)
	}()

	// Wait briefly to see if there's an immediate error (e.g., login required, invalid cluster)
	select {
	case err := <-errChan:
		// Command failed quickly - likely a configuration/validation error
		errorMessage := fmt.Sprintf("Failed to start console for cluster '%s'. Error: %v", clusterID, err)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: errorMessage},
			},
		}, nil, nil
	case <-time.After(5 * time.Second):
		// No immediate error - console is starting up successfully
		// The goroutine continues running in the background
	}

	// Build success message
	var successMessage strings.Builder
	successMessage.WriteString(fmt.Sprintf("✅ Console is starting for cluster '%s'\n\n", clusterID))
	successMessage.WriteString("🌐 Console will open in your default browser when ready\n\n")
	successMessage.WriteString("⚠️  IMPORTANT:\n")
	successMessage.WriteString("- The console is running in the background\n")
	successMessage.WriteString("- To stop it, manually stop the containers:\n")
	successMessage.WriteString(fmt.Sprintf("  podman stop console-%s monitoring-plugin-%s\n", clusterID, clusterID))
	successMessage.WriteString(fmt.Sprintf("  OR: docker stop console-%s monitoring-plugin-%s", clusterID, clusterID))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: successMessage.String()},
		},
	}, nil, nil
}
