package testJob

import (
	"context"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	backplaneApi "github.com/openshift/backplane-api/pkg/client"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/utils"
)

func newGetTestJobCommand() *cobra.Command {

	cmd := &cobra.Command{
		Use:           "get <testId>",
		Short:         "Get a backplane testjob resource",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runGetTestJob,
	}

	return cmd
}

func runGetTestJob(cmd *cobra.Command, args []string) error {
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
	isClusterHibernating, err := utils.DefaultOCMInterface.IsClusterHibernating(bpCluster.ClusterID)
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
	testId := args[0]

	client, err := utils.DefaultClientUtils.MakeRawBackplaneAPIClient(backplaneHost)
	if err != nil {
		return err
	}

	// ======== Call Endpoint ========
	resp, err := client.GetTestScriptRun(context.TODO(), clusterID, testId)

	// ======== Render Results ========
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return utils.TryPrintAPIError(resp, rawFlag)
	}

	createResp, err := backplaneApi.ParseGetTestScriptRunResponse(resp)

	if err != nil {
		return fmt.Errorf("unable to parse response body from backplane: \n Status Code: %d", resp.StatusCode)
	}

	fmt.Printf("TestId: %s, Status: %s\n", createResp.JSON200.TestId, *createResp.JSON200.Status)

	if rawFlag {
		_ = utils.RenderJsonBytes(createResp.JSON200)
	}
	return nil
}
