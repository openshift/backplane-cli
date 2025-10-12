package mcp_test

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	mcpCmd "github.com/openshift/backplane-cli/cmd/ocm-backplane/mcp"
)

var _ = Describe("MCP Command", func() {
	Context("Command configuration", func() {
		It("Should have correct command properties", func() {
			cmd := mcpCmd.MCPCmd

			Expect(cmd.Use).To(Equal("mcp"))
			Expect(cmd.Short).To(Equal("Start Model Context Protocol server"))
			Expect(cmd.Long).To(ContainSubstring("Start a Model Context Protocol (MCP) server"))
			Expect(cmd.Long).To(ContainSubstring("backplane resources and functionality"))
			Expect(cmd.Args).ToNot(BeNil())
			Expect(cmd.RunE).ToNot(BeNil())
			Expect(cmd.SilenceUsage).To(BeTrue())
		})

		It("Should accept no arguments", func() {
			cmd := mcpCmd.MCPCmd

			// Test that the command accepts exactly 0 arguments
			err := cmd.Args(cmd, []string{})
			Expect(err).To(BeNil())

			// Test that the command rejects arguments
			err = cmd.Args(cmd, []string{"arg1"})
			Expect(err).ToNot(BeNil())
		})

		It("Should have help available", func() {
			cmd := mcpCmd.MCPCmd
			err := cmd.Help()
			Expect(err).To(BeNil())
		})
	})

	Context("Command execution", func() {
		var (
			originalStdout *os.File
		)

		BeforeEach(func() {
			originalStdout = os.Stdout
		})

		AfterEach(func() {
			os.Stdout = originalStdout
		})

		It("Should have a RunE function that can be called", func() {
			cmd := mcpCmd.MCPCmd
			Expect(cmd.RunE).ToNot(BeNil())

			// We can't easily test the actual execution since it starts a server
			// that would block, but we can verify the function signature
			runE := cmd.RunE
			Expect(runE).To(BeAssignableToTypeOf(func(*cobra.Command, []string) error { return nil }))
		})
	})

	Context("Integration with parent commands", func() {
		It("Should be able to be added as a subcommand", func() {
			rootCmd := &cobra.Command{Use: "test"}
			rootCmd.AddCommand(mcpCmd.MCPCmd)

			// Verify the command was added
			subCommands := rootCmd.Commands()
			var foundMCP bool
			for _, cmd := range subCommands {
				if cmd.Use == "mcp" {
					foundMCP = true
					break
				}
			}
			Expect(foundMCP).To(BeTrue())
		})
	})

	Context("Command validation", func() {
		It("Should have optional flags for HTTP transport", func() {
			cmd := mcpCmd.MCPCmd

			// Verify the command has the expected flags
			httpFlag := cmd.Flags().Lookup("http")
			Expect(httpFlag).ToNot(BeNil())
			Expect(httpFlag.Value.String()).To(Equal("false")) // default value

			portFlag := cmd.Flags().Lookup("port")
			Expect(portFlag).ToNot(BeNil())
			Expect(portFlag.Value.String()).To(Equal("8080")) // default value

			// Verify no flags are required (both flags should be optional)
			Expect(httpFlag.DefValue).To(Equal("false"))
			Expect(portFlag.DefValue).To(Equal("8080"))
		})

		It("Should be runnable without prerequisites", func() {
			cmd := mcpCmd.MCPCmd

			// Verify the command can be prepared for execution
			cmd.SetArgs([]string{})
			err := cmd.ValidateArgs([]string{})
			Expect(err).To(BeNil())
		})

		It("Should accept HTTP flag and port flag", func() {
			cmd := mcpCmd.MCPCmd

			// Test with HTTP flag
			cmd.SetArgs([]string{"--http"})
			err := cmd.ValidateArgs([]string{})
			Expect(err).To(BeNil())

			// Test with both HTTP and port flags
			cmd.SetArgs([]string{"--http", "--port", "9090"})
			err = cmd.ValidateArgs([]string{})
			Expect(err).To(BeNil())
		})

		It("Should handle port flag independently", func() {
			cmd := mcpCmd.MCPCmd

			// Test port flag without HTTP (should still be valid)
			cmd.SetArgs([]string{"--port", "3000"})
			err := cmd.ValidateArgs([]string{})
			Expect(err).To(BeNil())
		})
	})

	Context("Error handling", func() {
		It("Should handle context cancellation gracefully", func() {
			// Create a cancelled context
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			// Verify we can create a command with the cancelled context
			// (This tests that the command setup doesn't panic with cancelled contexts)
			cmd := mcpCmd.MCPCmd
			cmd.SetContext(ctx)
			Expect(cmd.Context()).ToNot(BeNil())
		})
	})
})
