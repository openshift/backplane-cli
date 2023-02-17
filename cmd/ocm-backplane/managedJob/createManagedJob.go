package managedJob

import (
	"context"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	BackplaneApi "github.com/openshift/backplane-api/pkg/client"
	"github.com/openshift/backplane-cli/pkg/utils"
)

func newCreateManagedJobCmd() *cobra.Command {

	cmd := &cobra.Command{
		Use:           "create <script name>",
		Short:         "Creates a backplane managedjob resource",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runCreateManagedJob,
	}

	cmd.Flags().StringArrayP(
		"params",
		"p",
		[]string{},
		"Params to be passed to managedjob execution in json format. For e.g. -p 'VAR1=VAL1' -p VAR2=VAL2 ")

	return cmd
}

func runCreateManagedJob(cmd *cobra.Command, args []string) error {
	// ======== Parsing Flags ========
	// Params flag
	arr, err := cmd.Flags().GetStringArray("params")
	if err != nil {
		return err
	}

	parsedParams, err := utils.ParseParamsFlag(arr)
	if err != nil {
		return err
	}

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
	bpCluster, err := utils.GetBackplaneCluster(clusterKey)
	if err != nil {
		return err
	}

	// Check if the cluster is hibernating
	isClusterHibernating, err := utils.DefaultOCMInterface.IsClusterHibernating(bpCluster.ClusterID)
	if err == nil && isClusterHibernating {
		// Hibernating, print out error and skip
		return fmt.Errorf("cluster %s is hibernating, not creating ManagedJob", bpCluster.ClusterID)
	}

	backplaneHost := bpCluster.BackplaneHost
	clusterID := bpCluster.ClusterID

	if urlFlag != "" {
		backplaneHost = urlFlag
	}
	// ======== Parsing Args ========
	if len(args) < 1 {
		return fmt.Errorf("please input canonical-name of script as an argument. Please refer to \"ocm-backplane script list\"")
	}
	// It is always 1 in length, enforced by cobra
	canonicalNameArg := args[0]

	if err != nil {
		return err
	}

	client, err := utils.DefaultClientUtils.MakeRawBackplaneAPIClient(backplaneHost)
	if err != nil {
		return err
	}

	cj := BackplaneApi.CreateJobJSONRequestBody{
		CanonicalName: &canonicalNameArg,
		Parameters: &BackplaneApi.CreateJob_Parameters{
			AdditionalProperties: parsedParams,
		},
	}

	// ======== Call Endpoint ========
	resp, err := client.CreateJob(context.TODO(), clusterID, cj)

	// ======== Render Results ========
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return utils.TryPrintAPIError(resp, rawFlag)
	}

	createResp, err := BackplaneApi.ParseCreateJobResponse(resp)

	if err != nil {
		return fmt.Errorf("unable to parse response body from backplane: \n Status Code: %d", resp.StatusCode)
	}

	fmt.Printf("%s\nJobId: %s\n", *createResp.JSON200.Message, *createResp.JSON200.JobId)
	if rawFlag {
		_ = utils.RenderJsonBytes(createResp.JSON200)
	}
	return nil
}

func init() {

}
