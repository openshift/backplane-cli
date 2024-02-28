package managedjob

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"

	BackplaneApi "github.com/openshift/backplane-api/pkg/client"

	"github.com/openshift/backplane-cli/pkg/backplaneapi"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/ocm"
	"github.com/openshift/backplane-cli/pkg/utils"
)

func newLogsManagedJobCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "logs <job name>",
		Short:        "Get logs of a managedjob",
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

			// raw flag
			rawFlag, err := cmd.Flags().GetBool("raw")
			if err != nil {
				return err
			}
			logFlag, err := cmd.Flags().GetBool("follow")
			if err != nil {
				return err
			}

			managerFlag, err := cmd.Flags().GetBool("manager")
			if err != nil {
				return err
			}

			// ======== Parsing Args ========
			if len(args) < 1 {
				return fmt.Errorf("missing managedjob name as an argument")
			}
			managedJobName := args[0]

			// ======== Initialize backplaneURL ========
			bpConfig, err := config.GetBackplaneConfiguration()
			if err != nil {
				return err
			}

			bpCluster, err := utils.DefaultClusterUtils.GetBackplaneCluster(clusterKey)
			if err != nil {
				return err
			}

			if managerFlag {
				if mcid, clusterName, _, err := ocm.DefaultOCMInterface.GetManagingCluster(bpCluster.ClusterID); err == nil {
					bpCluster, err = utils.DefaultClusterUtils.GetBackplaneCluster(mcid)
					_ = clusterName
					if err != nil {
						return err
					}
				}
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
			version := "v2"
			resp, err := client.GetJobLogs(context.TODO(), clusterID, managedJobName, &BackplaneApi.GetJobLogsParams{Version: &version, Follow: &logFlag})

			// ======== Render Results ========
			if err != nil {
				return err
			}

			if resp.StatusCode != http.StatusOK {
				return utils.TryPrintAPIError(resp, rawFlag)
			}

			_, err = io.Copy(os.Stdout, resp.Body)
			if err != nil {
				return err
			}

			return nil
		},
	}
	cmd.PersistentFlags().BoolP("follow", "f", false, "Specify if logs should be streamed")
	cmd.PersistentFlags().Bool("manager", false, "Fetch the logs directly from the hive/MC")
	return cmd
}
