package mcp

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	mcptools "github.com/openshift/backplane-cli/pkg/ai/mcp"
)

// MCPCmd represents the mcp command
var MCPCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start Model Context Protocol server",
	Long: `Start a Model Context Protocol (MCP) server that provides access to backplane resources and functionality.

The MCP server allows AI assistants to interact with backplane clusters, retrieve status information,
and perform operations through the Model Context Protocol standard.`,
	Args:         cobra.ExactArgs(0),
	RunE:         runMCP,
	SilenceUsage: true,
}

func init() {
}

func runMCP(cmd *cobra.Command, argv []string) error {
	// Create a server with a single tool.
	server := mcp.NewServer(&mcp.Implementation{Name: "backplane-mcp", Version: "v1.0.0"}, nil)

	// Add the backplane-info tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "backplane_info",
		Description: "Get information about the current backplane CLI installation, configuration",
	}, mcptools.GetBackplaneInfo)

	// Run the server over stdin/stdout, until the client disconnects.
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		return err
	}

	return nil
}
