package cloud

import (
	"github.com/spf13/cobra"
)

var CloudCmd *cobra.Command = &cobra.Command{
	Use:               "cloud",
	Short:             "Cloud Access and Subcommands",
	Args:              cobra.NoArgs,
	DisableAutoGenTag: true,
	Run:               help,
}

func init() {
	CloudCmd.AddCommand(CredentialsCmd)
	CloudCmd.AddCommand(ConsoleCmd)
}

func help(cmd *cobra.Command, _ []string) {
	_ = cmd.Help()
}
