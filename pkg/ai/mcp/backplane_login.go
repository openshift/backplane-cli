package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/login"
)

type BackplaneLoginArgs struct {
	ClusterID string `json:"clusterId" jsonschema:"description:the cluster ID for backplane login"`
}

func BackplaneLogin(ctx context.Context, request *mcp.CallToolRequest, input BackplaneLoginArgs) (*mcp.CallToolResult, any, error) {
	clusterID := strings.TrimSpace(input.ClusterID)
	if clusterID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Error: Cluster ID is required for backplane login"},
			},
		}, nil, fmt.Errorf("cluster ID cannot be empty")
	}

	// Call the runLogin function directly instead of using exec
	err := login.LoginCmd.RunE(login.LoginCmd, []string{clusterID})

	if err != nil {
		errorMessage := fmt.Sprintf("Failed to login to cluster '%s'. Error: %v", clusterID, err)

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: errorMessage},
			},
		}, nil, nil // Return nil error since we're handling it gracefully
	}

	// Success case
	successMessage := fmt.Sprintf("Successfully logged in to cluster '%s'", clusterID)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: successMessage},
		},
	}, nil, nil
}
