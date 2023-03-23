package testJob

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-yaml/yaml"
	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	backplaneApi "github.com/openshift/backplane-api/pkg/client"
	"github.com/openshift/backplane-cli/pkg/utils"
)

func newCreateTestJobCommand() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a backplane test job",
		Long: `
Create a test job on a non-production cluster

** NOTE: For testing scripts only **

This command will assume that you are already in a managed-script directory, 
eg: https://github.com/openshift/managed-scripts/tree/main/scripts/SREP/example

When running this command, it will attempt to read "metadata.yaml" and the script file specified and create a 
test job run without having to commit to the upstream managed-script repository

By default, the container image used to run your script will be the latest image built via the managed-script
github repository

Example usage:
  cd scripts/SREP/example && ocm backplane testjob create -p var1=val1

`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runCreateTestJob,
	}

	cmd.Flags().StringArrayP(
		"params",
		"p",
		[]string{},
		"Params to be passed to managedjob execution in json format. For e.g. -p 'VAR1=VAL1' -p VAR2=VAL2 ")

	return cmd
}

func runCreateTestJob(cmd *cobra.Command, args []string) error {
	isProd, err := utils.DefaultOCMInterface.IsProduction()
	if err != nil {
		return err
	}
	if isProd {
		return fmt.Errorf("testjob can not be used in production environment")
	}

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

	if err != nil {
		return err
	}
	backplaneHost, err := utils.DefaultOCMInterface.GetBackplaneURL()
	if err != nil {
		return err
	}
	clusterID := bpCluster.ClusterID
	

	if urlFlag != "" {
		backplaneHost = urlFlag
	}

	client, err := utils.DefaultClientUtils.MakeRawBackplaneAPIClient(backplaneHost)
	if err != nil {
		return err
	}

	cj, err := createTestScriptFromFiles()
	if err != nil {
		return err
	}

	cj.Parameters = &backplaneApi.CreateTestJob_Parameters{AdditionalProperties: parsedParams}

	// ======== Call Endpoint ========
	resp, err := client.CreateTestScriptRun(context.TODO(), clusterID, *cj)

	// ======== Render Results ========
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return utils.TryPrintAPIError(resp, rawFlag)
	}

	createResp, err := backplaneApi.ParseCreateTestScriptRunResponse(resp)

	if err != nil {
		return fmt.Errorf("unable to parse response body from backplane: \n Status Code: %d", resp.StatusCode)
	}

	fmt.Printf("%s\nTestId: %s\n", *createResp.JSON200.Message, createResp.JSON200.TestId)
	if rawFlag {
		_ = utils.RenderJsonBytes(createResp.JSON200)
	}
	return nil
}

func createTestScriptFromFiles() (*backplaneApi.CreateTestScriptRunJSONRequestBody, error) {
	// Get the current directory
	dir, err := os.Getwd()
	if err != nil {
		logger.Errorf("Error getting current working directory: %v", err)
		return nil, err
	}

	// Read the yaml file from cwd
	yamlFile, err := os.ReadFile(filepath.Join(dir, "metadata.yaml"))
	if err != nil {
		logger.Errorf("Error reading metadata yaml: %v, ensure you are in a script directory", err)
		return nil, err
	}

	scriptMeta := backplaneApi.ScriptMetadata{}

	err = yaml.Unmarshal(yamlFile, &scriptMeta)
	if err != nil {
		logger.Errorf("Error reading metadata: %v", err)
		return nil, err
	}

	// Now, try to find and read the script body
	scriptFile := filepath.Join(dir, scriptMeta.File)
	fileBody, err := os.ReadFile(scriptFile)
	if err != nil {
		logger.Errorf("Error reading file %s: %v", scriptFile, err)
		return nil, err
	}

	// Base64 encode the body
	scriptBodyEncoded := base64.StdEncoding.EncodeToString(fileBody)

	return &backplaneApi.CreateTestScriptRunJSONRequestBody{
		ScriptBody:     scriptBodyEncoded,
		ScriptMetadata: scriptMeta,
	}, nil
}
