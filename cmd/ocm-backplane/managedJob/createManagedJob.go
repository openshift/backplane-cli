package managedjob

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	BackplaneApi "github.com/openshift/backplane-api/pkg/client"
	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/openshift/backplane-cli/pkg/backplaneapi"
	"github.com/openshift/backplane-cli/pkg/ocm"
	"github.com/openshift/backplane-cli/pkg/utils"
)

var (
	options struct {
		canonicalName string
		params        []string
		wait          bool
		clusterID     string
		url           string
		raw           bool
		logs          bool
		manager       bool
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

	cmd.Flags().BoolVarP(
		&options.logs,
		"logs",
		"",
		false,
		"Fetch logs from the pod for the running job")

	cmd.Flags().BoolVarP(
		&options.manager,
		"manager",
		"",
		false,
		"Run the job on manager/hive shard if flag is set --manager")

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
	bpCluster, err := utils.DefaultClusterUtils.GetBackplaneCluster(options.clusterID)
	if err != nil {
		return err
	}

	if options.manager {
		if mcid, clusterName, _, err := ocm.DefaultOCMInterface.GetManagingCluster(bpCluster.ClusterID); err == nil {
			bpCluster, err = utils.DefaultClusterUtils.GetBackplaneCluster(mcid)
			if err != nil {
				return err
			}
		} else {
			c, err := ocm.DefaultOCMInterface.GetClusterInfoByID(bpCluster.ClusterID)
			if err != nil {
				return err
			}
			p, ok := c.GetProduct()
			if !ok {
				return fmt.Errorf("could not get product information")
			}
			return fmt.Errorf("product id is %s and bplane url is %s for cluster: %s\nThe feature is not available for OSD and ROSA, when not using in PRODUCTION", p.ID(), bpCluster.ClusterURL, clusterName)
		}
	}

	// Check if the cluster is hibernating
	isClusterHibernating, err := ocm.DefaultOCMInterface.IsClusterHibernating(bpCluster.ClusterID)
	if err == nil && isClusterHibernating {
		// Hibernating, print out error and skip
		return fmt.Errorf("cluster %s is hibernating, not creating ManagedJob", bpCluster.ClusterID)
	}

	backplaneHost := bpCluster.BackplaneHost
	options.clusterID = bpCluster.ClusterID
	if options.url != "" {
		backplaneHost = options.url
	}

	// get raw backplane API client
	client, err := backplaneapi.DefaultClientUtils.MakeRawBackplaneAPIClient(backplaneHost)
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

	// stream logs if flag set
	if options.logs {
		fmt.Fprintf(cmd.OutOrStdout(), "fetching logs for job %s", *job.JobId)
		err := fetchJobLogs(client, job)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "")
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
	options.clusterID, err = cmd.Flags().GetString("cluster-id")
	if err != nil {
		return err
	}

	// init URL flag
	options.url, err = cmd.Flags().GetString("url")
	if err != nil {
		return err
	}
	if options.url != "" {
		parsedURL, parseErr := url.ParseRequestURI(options.url)
		if parseErr != nil {
			return fmt.Errorf("invalid --url: %v", parseErr)
		}
		if parsedURL.Scheme != "https" {
			return fmt.Errorf("invalid --url '%s': scheme must be https", options.url)
		}
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
		Parameters:    &jobParams,
	}

	// call create end point
	resp, err := client.CreateJob(context.TODO(), options.clusterID, createJob)

	if err != nil {
		return nil, err
	}

	// Check for the warning header and display it if found.
	if warningMsg := resp.Header.Get("Backplane-Warning"); warningMsg != "" {
		logger.Warnf("warning: %s", warningMsg)
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
		_ = utils.RenderJSONBytes(createResp.JSON200)
	}
	return createResp.JSON200, nil
}

// waitForCreateJob wait until job status to be Succeeded
// waitForCreateJob timeouts after 10 min
func waitForCreateJob(client BackplaneApi.ClientInterface, job *BackplaneApi.Job) (statusMessage string, err error) {

	pollErr := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, time.Duration(600)*time.Second, true, func(context.Context) (bool, error) {
		fmt.Printf(".")

		// Get the current job
		status, err := getJobStatus(client, job)
		if err != nil {
			return false, err
		}

		// Check if the job is in the expected status
		if status == BackplaneApi.JobStatusStatusSucceeded {
			statusMessage = "Job Succeeded"
			return true, nil
		}
		if status == BackplaneApi.JobStatusStatusFailed {
			statusMessage = "Job Failed"
			return true, nil
		}

		return false, nil
	})

	return statusMessage, pollErr
}

// fetchJobLogs stream the log of the job to the console output when the job status is Running, Succeeded or Failed
func fetchJobLogs(client BackplaneApi.ClientInterface, job *BackplaneApi.Job) error {

	pollErr := wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 1*time.Minute, true, func(context.Context) (bool, error) {
		fmt.Printf(".")

		// Get the current job
		status, err := getJobStatus(client, job)
		if err != nil {
			return false, err
		}

		// Check if the job is in the expected status
		switch status {
		case BackplaneApi.JobStatusStatusPending:
			return false, nil
		case BackplaneApi.JobStatusStatusRunning:
			fmt.Println("")
			return true, nil
		case BackplaneApi.JobStatusStatusSucceeded:
			fmt.Println("")
			return true, nil
		case BackplaneApi.JobStatusStatusFailed:
			fmt.Println("")
			return true, nil
		default:
			return false, fmt.Errorf("job is not ready with logs")
		}

	})

	if pollErr != nil {
		return pollErr
	}

	version := "v2"
	follow := true

	resp, err := client.GetJobLogs(context.TODO(), options.clusterID, *job.JobId, &BackplaneApi.GetJobLogsParams{Version: &version, Follow: &follow})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return utils.TryPrintAPIError(resp, true)
	}

	_, err = io.Copy(os.Stdout, resp.Body)
	if err != nil {
		return err
	}

	return err
}

func getJobStatus(client BackplaneApi.ClientInterface, job *BackplaneApi.Job) (BackplaneApi.JobStatusStatus, error) {
	jobResp, err := client.GetRun(context.TODO(), options.clusterID, *job.JobId)

	if err != nil {
		return "", err
	}

	formatJobResp, err := BackplaneApi.ParseGetRunResponse(jobResp)

	if err != nil {
		return "", err
	}

	return *formatJobResp.JSON200.JobStatus.Status, nil
}