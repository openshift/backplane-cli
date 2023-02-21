package testJob

import (
	"github.com/spf13/cobra"
)

func NewTestJobCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "testjob",
		Aliases:      []string{"testJob", "testjobs", "tj"},
		Short:        "Represents a backplane testJob.",
		SilenceUsage: true,
	}

	// url flag
	// Denotes backplane url
	// If this flag is empty, backplane-url will be fetched by user settings.
	cmd.PersistentFlags().String(
		"url",
		"",
		"Specify backplane url.",
	)

	// cluster-id Flag
	cmd.PersistentFlags().StringP("cluster-id", "c", "", "Cluster ID could be cluster name, id or external-id")

	// raw Flag
	cmd.PersistentFlags().Bool("raw", false, "Prints the raw response returned by the backplane API")

	cmd.AddCommand(
		newCreateTestJobCommand(),
		newGetTestJobCommand(),
		newGetTestJobLogsCommand(),
	)

	return cmd
}
