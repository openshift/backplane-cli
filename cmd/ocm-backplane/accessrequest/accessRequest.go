package accessrequest

import (
	"github.com/spf13/cobra"
)

func NewAccessRequestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "accessrequest",
		Aliases:      []string{"accessRequest", "accessrequest", "accessrequests"},
		Short:        "Manages access requests for clusters on which access protection is enabled",
		SilenceUsage: true,
	}

	// cluster-id Flag
	cmd.PersistentFlags().StringP("cluster-id", "c", "", "Cluster ID could be cluster name, id or external-id")

	cmd.AddCommand(newCreateAccessRequestCmd())
	cmd.AddCommand(newGetAccessRequestCmd())
	cmd.AddCommand(newExpireAccessRequestCmd())

	return cmd
}

func init() {
}
