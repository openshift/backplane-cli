package remediation

import (
	"context"
	"fmt"
	"net/http"

	"github.com/openshift/backplane-cli/pkg/backplaneapi"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/utils"
	"github.com/spf13/cobra"
)

func NewRemediationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "remediation",
		Short:        "Remediation cleanup",
		SilenceUsage: true,
	}
	cmd.PersistentFlags().String(
		"url",
		"",
		"Specify backplane url.",
	)

	// cluster-id Flag
	cmd.PersistentFlags().StringP("cluster-id", "c", "", "Cluster ID could be cluster name, id or external-id")

	// raw Flag
	cmd.PersistentFlags().Bool("raw", false, "Prints the raw response returned by the backplane API")
	cmd.AddCommand(newDeleteRemediationCmd())
	return cmd
}

func newDeleteRemediationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "delete",
		Short:        "Delete remediation RBAC",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// ======== Parsing Flags ========
			// Cluster ID flag
			clusterKey, err := cmd.Flags().GetString("cluster-id")
			if err != nil {
				return err
			}

			// URL flag
			urlFlag, err := cmd.Flags().GetString("url")
			if err != nil {
				return err
			}

			// ======== Initialize backplaneURL ========
			bpConfig, err := config.GetBackplaneConfiguration()
			if err != nil {
				return err
			}

			bpCluster, err := utils.DefaultClusterUtils.GetBackplaneCluster(clusterKey)
			if err != nil {
				return err
			}

			backplaneHost := bpConfig.URL
			if err != nil {
				return err
			}
			clusterID := bpCluster.ClusterID

			if urlFlag != "" {
				backplaneHost = urlFlag
			}

			client, err := backplaneapi.DefaultClientUtils.MakeRawBackplaneAPIClient(backplaneHost)
			if err != nil {
				return err
			}

			// ======== Call Endpoint ========
			resp, err := client.DeleteRemediation(context.TODO(), clusterID)

			// ======== Render Results ========
			if err != nil {
				return err
			}

			if resp.StatusCode != http.StatusOK {
				return utils.TryPrintAPIError(resp, false)
			}

			fmt.Printf("Deleted remediations RBAC on cluster %s\n", clusterID)
			return nil
		},
	}
	return cmd
}
