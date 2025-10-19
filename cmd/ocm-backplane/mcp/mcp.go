package mcp

import (
	"context"
	"fmt"
	"net/http"
	"time"

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
	MCPCmd.Flags().Bool("http", false, "Run MCP server over HTTP instead of stdio")
	MCPCmd.Flags().Int("port", 8080, "Port to run HTTP server on (only used with --http)")
}

func runMCP(cmd *cobra.Command, argv []string) error {
	// Get flag values
	useHTTP, _ := cmd.Flags().GetBool("http")
	port, _ := cmd.Flags().GetInt("port")

	// Create a server with backplane tools.
	server := mcp.NewServer(&mcp.Implementation{Name: "backplane", Version: "v1.0.0"}, nil)

	// Add the info tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "info",
		Description: "Get information about the current backplane CLI installation, configuration",
	}, mcptools.GetBackplaneInfo)

	// Add the login tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "login",
		Description: "Login to cluster via backplane",
	}, mcptools.BackplaneLogin)

	// Add the console tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "console",
		Description: "Access cluster console via backplane CLI, optionally opening in browser",
	}, mcptools.BackplaneConsole)

	// Add the cluster resource tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "cluster-resource",
		Description: "Execute read-only Kubernetes resource operations (get, describe, logs, top, explain) on cluster resources",
	}, mcptools.BackplaneClusterResource)

	// Add the cloud console tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "cloud-console",
		Description: "Get cloud provider console access for a cluster with temporary credentials",
	}, mcptools.BackplaneCloudConsole)

	// Choose transport method based on flags
	if useHTTP {
		// HTTP Transport Mode
		// This mode runs the MCP server over HTTP using Server-Sent Events (SSE) streaming.
		//
		// IMPORTANT: This is NOT a standard REST API endpoint!
		// - The server uses SSE (Server-Sent Events) for bi-directional communication
		// - Direct HTTP POST requests with curl will NOT work
		// - This is designed for MCP clients that support HTTP SSE transport
		//
		// Use cases for HTTP transport:
		// 1. Web-based MCP clients that implement SSE protocol
		// 2. Custom applications using MCP client libraries with HTTP support
		// 3. Programmatic integration requiring HTTP-based MCP access
		//
		// For AI assistant integration (Claude Desktop, Gemini CLI), use stdio transport instead.
		// For testing, use the default stdio mode without the --http flag.

		addr := fmt.Sprintf(":%d", port)
		fmt.Printf("Starting MCP server on HTTP at http://localhost%s\n", addr)
		fmt.Printf("Note: This uses SSE streaming protocol, not standard REST API.\n")
		fmt.Printf("Direct curl requests will not work. Use MCP clients that support HTTP transport.\n")

		// Create HTTP handler that returns our server
		// The StreamableHTTPHandler manages SSE connections and message routing
		handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
			return server
		}, nil)

		httpServer := &http.Server{
			Addr:              addr,
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		}

		if err := httpServer.ListenAndServe(); err != nil {
			return fmt.Errorf("HTTP server error: %w", err)
		}
	} else {
		// Stdio Transport Mode (Default)
		// This is the standard mode for AI assistant integration.
		// The server communicates via stdin/stdout using JSON-RPC protocol.
		// This is the recommended mode for:
		// - Claude Desktop integration
		// - Gemini CLI integration
		// - Local command-line testing
		// - Any MCP client that supports stdio transport
		if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			return err
		}
	}

	return nil
}
