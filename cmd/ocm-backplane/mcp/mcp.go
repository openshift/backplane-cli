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
		// Run the server over HTTP using StreamableHTTPHandler
		addr := fmt.Sprintf(":%d", port)
		fmt.Printf("Starting MCP server on HTTP at http://localhost%s\n", addr)

		// Create HTTP handler that returns our server
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
		// Run the server over stdin/stdout, until the client disconnects.
		if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			return err
		}
	}

	return nil
}
