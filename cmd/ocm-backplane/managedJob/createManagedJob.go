package managedJob

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	BackplaneApi "github.com/openshift/backplane-api/pkg/client"
	"github.com/openshift/backplane-cli/pkg/utils"
	"k8s.io/apimachinery/pkg/util/wait"
)

var (
	options struct {
		canonicalName string
		params        []string
		wait          bool
		clusterId     string
		url           string
		raw           bool
	}
)

// newCreateManagedJobCmd returns cobra command
func newCreateManagedJobCmd() *cobra.Command {

	cmd := &cobra.Command{
		Use:           "create <script name>",
		Short:         "Creates a backplane managedjob resource",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runCreateManagedJob,
	}

	cmd.Flags().StringArrayVarP(
		&options.params,
		"params",
		"p",
		[]string{},
		"Params to be passed to managedjob execution in json format. For e.g. -p 'VAR1=VAL1' -p VAR2=VAL2 ")

	cmd.Flags().BoolVarP(
		&options.wait,
		"wait",
		"w",
		false,
		"Wait until command execution is finished")

	return cmd
}

// runCreateManagedJob creates managed job in the specific cluster
func runCreateManagedJob(cmd *cobra.Command, args []string) (err error) {

	// init params and validate
	err = initParams(cmd, args)

	if err != nil {
		return err
	}

	// ======== Initialize backplaneURL ========
	bpCluster, err := utils.DefaultClusterUtils.GetBackplaneCluster(options.clusterId)
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
	options.clusterId = clusterID

	if options.url != "" {
		backplaneHost = options.url
	}

	// get raw backplane API client
	client, err := utils.DefaultClientUtils.MakeRawBackplaneAPIClient(backplaneHost)
	if err != nil {
		return err
	}

	// create the job
	job, err := createJob(client)

	if err != nil {
		return err
	}

	// wait for job to be finished
	if options.wait {
		fmt.Fprintf(cmd.OutOrStdout(), "\nWaiting for %s to be finished ...", *job.JobId)
		statusMessage, err := waitForCreateJob(client, job)

		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n.", statusMessage)
	}

	return nil
}

// initParams initialize parameters and validate them
func initParams(cmd *cobra.Command, argv []string) (err error) {
	// validate job canonical name
	if len(argv) < 1 {
		return fmt.Errorf("please input canonical-name of script as an argument. Please refer to \"ocm-backplane script list\"")
	}

	// init job canonical name
	canonicalNameArg := argv[0]
	options.canonicalName = canonicalNameArg

	// init Cluster key
	options.clusterId, err = cmd.Flags().GetString("cluster-id")
	if err != nil {
		return err
	}

	// init URL flag
	options.url, err = cmd.Flags().GetString("url")
	if err != nil {
		return err
	}

	// init raw flag
	options.raw, err = cmd.Flags().GetBool("raw")
	if err != nil {
		return err
	}

	return nil
}

// createJob initializes the job creation in a specific cluster and returns the job info
func createJob(client BackplaneApi.ClientInterface) (*BackplaneApi.Job, error) {

	jobParams, err := utils.ParseParamsFlag(options.params)

	if err != nil {
		return nil, err
	}

	// create job request
	createJob := BackplaneApi.CreateJobJSONRequestBody{
		CanonicalName: &options.canonicalName,
		Parameters: &BackplaneApi.CreateJob_Parameters{
			AdditionalProperties: jobParams,
		},
	}

	// call create end point
	resp, err := client.CreateJob(context.TODO(), options.clusterId, createJob)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, utils.TryPrintAPIError(resp, options.raw)
	}

	// format create job response
	createResp, err := BackplaneApi.ParseCreateJobResponse(resp)

	if err != nil {
		return nil, fmt.Errorf("unable to parse response body from backplane: \n Status Code: %d", resp.StatusCode)
	}

	// render job details
	fmt.Printf("%s\nJobId: %s\n", *createResp.JSON200.Message, *createResp.JSON200.JobId)
	if options.raw {
		_ = utils.RenderJsonBytes(createResp.JSON200)
	}
	return createResp.JSON200, nil
}

// waitForCreateJob wait until job status to be Succeeded
// waitForCreateJob timeouts after 10 min
func waitForCreateJob(client BackplaneApi.ClientInterface, job *BackplaneApi.Job) (statusMessage string, err error) {

	pollErr := wait.PollImmediate(10*time.Second, time.Duration(600)*time.Second, func() (bool, error) {
		fmt.Printf(".")

		// ========= Get the current job ============
		jobResp, err := client.GetRun(context.TODO(), options.clusterId, *job.JobId)

		if err != nil {
			return false, err
		}

		formatJobResp, err := BackplaneApi.ParseGetRunResponse(jobResp)

		if err != nil {
			return false, err
		}

		if *formatJobResp.JSON200.JobStatus.Status == BackplaneApi.JobStatusStatusSucceeded {
			statusMessage = "Job Succeeded"
			return true, nil
		}

		if *formatJobResp.JSON200.JobStatus.Status == BackplaneApi.JobStatusStatusFailed {
			statusMessage = "Job Failed"
			return true, nil
		}

		return false, err
	})

	return statusMessage, pollErr
}
