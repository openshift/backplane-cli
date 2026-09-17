package testjob

import (
	"github.com/spf13/cobra"
)

func NewTestJobCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "testjob",
		Aliases:      []string{"testJob", "testjobs", "tj"},
		Short:        "Represents a backplane testJob.",
		SilenceUsage: true,
		Hidden:       true,
	}

	cmd.AddCommand(
		newRenderTestJobCommand(),
	)

	return cmd
}
