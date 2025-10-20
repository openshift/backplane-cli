package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/cloud"
)

type BackplaneCloudConsoleArgs struct {
	ClusterID     string `json:"clusterId" jsonschema:"description:the cluster ID for backplane cloud console"`
	OpenInBrowser bool   `json:"openInBrowser,omitempty" jsonschema:"description:whether to open the cloud console URL in a browser"`
	Output        string `json:"output,omitempty" jsonschema:"description:output format, e.g. 'text' or 'json'"`
	URL           string `json:"url,omitempty" jsonschema:"description:override backplane API URL (otherwise use env)"`
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

	// Configure flags
	if input.OpenInBrowser {
		err := consoleCmd.Flags().Set("browser", "true")
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Error setting browser flag: %v", err)},
				},
			}, nil, nil
		}
	}

	// Set output format (default to json for parsing)
	output := input.Output
	if output == "" {
		output = "json"
	}
	err := consoleCmd.Flags().Set("output", output)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error setting output flag: %v", err)},
			},
		}, nil, nil
	}

	// Set URL if provided
	if input.URL != "" {
		err := consoleCmd.Flags().Set("url", input.URL)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Error setting URL flag: %v", err)},
				},
			}, nil, nil
		}
	}

	// Call the cloud console command's RunE function directly
	err = consoleCmd.RunE(consoleCmd, args)

	if err != nil {
		errorMessage := fmt.Sprintf("Failed to get cloud console for cluster '%s'. Error: %v", clusterID, err)

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: errorMessage},
			},
		}, nil, nil
	}

	// Build success message
	var successMessage strings.Builder
	successMessage.WriteString(fmt.Sprintf("Successfully retrieved cloud console access for cluster '%s'", clusterID))

	if input.OpenInBrowser {
		successMessage.WriteString("\n🌐 Cloud console opened in default browser")
	}

	if output != "text" {
		successMessage.WriteString(fmt.Sprintf("\n📋 Output format: %s", output))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: successMessage.String()},
		},
	}, nil, nil
}
