package mcp

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/console"
)

type BackplaneConsoleArgs struct {
	ClusterID     string `json:"clusterId" jsonschema:"description:the cluster ID for backplane console"`
	OpenInBrowser bool   `json:"openInBrowser,omitempty" jsonschema:"description:whether to automatically open the console URL in browser"`
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

	// Configure flags if needed
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

	// Capture stdout to get the console URL
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Channel to capture the command execution result
	resultChan := make(chan error, 1)

	// Run the console command in a goroutine
	go func() {
		resultChan <- consoleCmd.RunE(consoleCmd, args)
	}()

	// Capture output with timeout to get the console URL
	outputChan := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			buf.WriteString(line + "\n")
			// Once we see the console URL, we have what we need
			if strings.Contains(line, "Console is available at") {
				outputChan <- buf.String()
				return
			}
		}
		outputChan <- buf.String()
	}()

	var output string
	var consoleURL string

	// Wait for output with timeout
	select {
	case output = <-outputChan:
		// Got the console URL
	case <-time.After(30 * time.Second):
		// Timeout waiting for console URL
		output = "Console is starting (timeout waiting for URL)..."
	}

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout
	io.Copy(io.Discard, r) // Drain the pipe

	// Check for immediate errors (non-blocking)
	select {
	case err := <-resultChan:
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Failed to start console for cluster '%s'. Error: %v", clusterID, err)},
				},
			}, nil, nil
		}
	default:
		// Command is still running in background - this is expected
	}

	// Extract console URL from output
	// The actual output format is: "== Console is available at http://127.0.0.1:PORT =="
	urlRegex := regexp.MustCompile(`Console is available at (https?://[^\s=]+)`)
	matches := urlRegex.FindStringSubmatch(output)
	if len(matches) > 1 {
		consoleURL = matches[1]
	}

	// Build success message
	var successMessage strings.Builder
	successMessage.WriteString(fmt.Sprintf("Successfully started cluster console for '%s'", clusterID))

	if consoleURL != "" {
		successMessage.WriteString(fmt.Sprintf("\n🔗 Console URL: %s", consoleURL))
	}

	if input.OpenInBrowser {
		successMessage.WriteString("\n🌐 Console opened in default browser")
	}

	successMessage.WriteString("\n\nℹ️  Console is running in the background. It will continue to run until manually stopped.")

	if output != "" {
		successMessage.WriteString(fmt.Sprintf("\n\nInitial output:\n%s", strings.TrimSpace(output)))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: successMessage.String()},
		},
	}, nil, nil
}
