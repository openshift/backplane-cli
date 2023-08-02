package cloud

import (
	bpconfig "github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/spf13/cobra"
)

var GetBackplaneConfiguration = bpconfig.GetBackplaneConfiguration

var CloudCmd = &cobra.Command{
	Use:               "cloud",
	Short:             "Cluster cloud provider access",
	Args:              cobra.NoArgs,
	DisableAutoGenTag: true,
	Run:               help,
}

func init() {
	CloudCmd.AddCommand(CredentialsCmd)
	CloudCmd.AddCommand(ConsoleCmd)
	CloudCmd.AddCommand(TokenCmd)
	CloudCmd.AddCommand(AssumeCmd)
}

func help(cmd *cobra.Command, _ []string) {
	_ = cmd.Help()
}
