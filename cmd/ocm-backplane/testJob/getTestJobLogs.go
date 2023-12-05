package testjob

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"

	bpClient "github.com/openshift/backplane-api/pkg/client"

	"github.com/openshift/backplane-cli/pkg/backplaneapi"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/ocm"
	"github.com/openshift/backplane-cli/pkg/utils"
)

func newGetTestJobLogsCommand() *cobra.Command {

	cmd := &cobra.Command{
		Use:           "logs <testId>",
		Aliases:       []string{"log"},
		Short:         "Get a backplane testJob logs",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runGetTestJobLogs,
	}

	return cmd
}

func runGetTestJobLogs(cmd *cobra.Command, args []string) error {
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
	// ======== Initialize backplaneURL ========
	bpConfig, err := config.GetBackplaneConfiguration()
	if err != nil {
		return err
	}

	bpCluster, err := utils.DefaultClusterUtils.GetBackplaneCluster(clusterKey)
	if err != nil {
		return err
	}

	// Check if the cluster is hibernating
	isClusterHibernating, err := ocm.DefaultOCMInterface.IsClusterHibernating(bpCluster.ClusterID)
	if err == nil && isClusterHibernating {
		// Hibernating, print out error and skip
		return fmt.Errorf("cluster %s is hibernating, not creating ManagedJob", bpCluster.ClusterID)
	}

	backplaneHost := bpConfig.URL

	clusterID := bpCluster.ClusterID

	if urlFlag != "" {
		backplaneHost = urlFlag
	}

	// It is always 1 in length, enforced by cobra
	testID := args[0]

	client, err := backplaneapi.DefaultClientUtils.MakeRawBackplaneAPIClient(backplaneHost)
	if err != nil {
		return err
	}

	// ======== Call Endpoint ========
	version := "v2"
	resp, err := client.GetTestScriptRunLogs(context.TODO(), clusterID, testID, &bpClient.GetTestScriptRunLogsParams{Version: &version, Follow: &logFlag})

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
}
