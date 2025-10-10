package mcp

import (
	"context"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/info"
)

// BackplaneInfoInput represents the input for the backplane-info tool
type BackplaneInfoInput struct {
	// No input parameters needed for backplane info
}

// GetBackplaneInfo retrieves comprehensive information about the backplane CLI installation
// and configuration
func GetBackplaneInfo(ctx context.Context, req *mcp.CallToolRequest, input BackplaneInfoInput) (*mcp.CallToolResult, any, error) {
	// Get version information
	version := info.DefaultInfoService.GetVersion()

	// Get configuration information
	bpConfig, err := config.GetBackplaneConfiguration()
	var configInfo string
	if err != nil {
		configInfo = fmt.Sprintf("Error loading configuration: %v", err)
	} else {
		// Helper function logic inlined
		sessionDir := bpConfig.SessionDirectory
		if sessionDir == "" {
			sessionDir = info.BackplaneDefaultSessionDirectory
		}

		proxyURL := "not configured"
		if bpConfig.ProxyURL != nil && *bpConfig.ProxyURL != "" {
			proxyURL = *bpConfig.ProxyURL
		}

		awsProxy := "not configured"
		if bpConfig.AwsProxy != nil && *bpConfig.AwsProxy != "" {
			awsProxy = *bpConfig.AwsProxy
		}

		configInfo = fmt.Sprintf(`Configuration:
- Backplane URL: %s
- Session Directory: %s
- Proxy URL: %s
- AWS Proxy: %s
- Display Cluster Info: %t
- GovCloud: %t`,
			bpConfig.URL,
			sessionDir,
			proxyURL,
			awsProxy,
			bpConfig.DisplayClusterInfo,
			bpConfig.Govcloud)
	}

	// Get current working directory and environment info
	cwd, _ := os.Getwd()
	homeDir, _ := os.UserHomeDir()

	// Build complete info response
	infoText := fmt.Sprintf(`Backplane CLI Information:

Version: %s

%s

Environment:
- Home Directory: %s
- Current Directory: %s
- Shell: %s`, version, configInfo, homeDir, cwd, os.Getenv("SHELL"))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: infoText},
		},
	}, nil, nil
}
