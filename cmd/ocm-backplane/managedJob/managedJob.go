package managedJob

import (
	"github.com/spf13/cobra"
)

func NewManagedJobCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "managedjob",
		Aliases:      []string{"managedJob", "managedjob", "managedjobs"},
		Short:        "Represents a backplane managedjob.",
		SilenceUsage: true,
	}

	// url flag
	// Denotes backplane url
	// If this flag is empty, backplane-url will be fetched by the user local settings. either via BACKPLANE_URL env or ~/backplane.{env}.json file
	cmd.PersistentFlags().String(
		"url",
		"",
		"Specify backplane url.",
	)

	// cluster-id Flag
	cmd.PersistentFlags().StringP("cluster-id", "c", "", "Cluster ID could be cluster name, id or external-id")

	// raw Flag
	cmd.PersistentFlags().Bool("raw", false, "Prints the raw response returned by the backplane API")

	cmd.AddCommand(newCreateManagedJobCmd(), newGetManagedJobCmd(), newDeleteManagedJobCmd(), newLogsManagedJobCmd())

	return cmd
}

func init() {

}
