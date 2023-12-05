package testjob

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	backplaneApi "github.com/openshift/backplane-api/pkg/client"

	"github.com/openshift/backplane-cli/pkg/backplaneapi"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/ocm"
	"github.com/openshift/backplane-cli/pkg/utils"
)

var GetGitRepoPath = exec.Command("git", "rev-parse", "--show-toplevel")

func newCreateTestJobCommand() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a backplane test job",
		Long: `
Create a test job on a non-production cluster

** NOTE: For testing scripts only **

This command will assume that you are already in a managed-scripts directory,
eg: https://github.com/openshift/managed-scripts/tree/main/scripts/SREP/example

When running this command, it will attempt to read "metadata.yaml" and the script file specified and create a
test job run without having to commit to the upstream managed-script repository

By default, the container image used to run your script will be the latest image built via the managed-script
github repository

To use with bash libraries, make sure the libraries are in the scripts directory of your managed scripts repository, in the format: source /managed-scripts/<path-from-managed-scripts-scripts-dir>.

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
		"Params to be passed to managedjob execution in json format. Example: -p 'VAR1=VAL1' -p VAR2=VAL2 ")

	cmd.Flags().StringP(
		"library-file-path",
		"l",
		"",
		"Optional library file to be passed in (must live in managed-scripts/scripts directory)",
	)

	cmd.Flags().BoolP(
		"dry-run",
		"d",
		false,
		"Use this flag to perform a dry run, which will yield the YAML of the job without creating it.",
	)

	cmd.Flags().StringP(
		"base-image-override",
		"i",
		"",
		"Optional custom repository URI to override managed-scripts base image. Example: base-image-override=quay.io/foobar/managed-scripts:latest.",
	)

	return cmd
}

func runCreateTestJob(cmd *cobra.Command, args []string) error {
	isProd, err := ocm.DefaultOCMInterface.IsProduction()
	if err != nil {
		return err
	}
	if isProd {
		return fmt.Errorf("testjob can not be used in production environment")
	}

	// ======== Parsing Flags ========
	// Params flag
	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		return err
	}

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

	// Base image override flag
	baseImageOverrideFlag, err := cmd.Flags().GetString("base-image-override")
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
	isClusterHibernating, err := ocm.DefaultOCMInterface.IsClusterHibernating(bpCluster.ClusterID)
	if err == nil && isClusterHibernating {
		// Hibernating, print out error and skip
		return fmt.Errorf("cluster %s is hibernating, not creating ManagedJob", bpCluster.ClusterID)
	}

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

	cj, err := createTestScriptFromFiles(dryRun)
	if err != nil {
		return err
	}

	if baseImageOverrideFlag != "" {
		cj.BaseImageOverride = &baseImageOverrideFlag
	}

	cj.Parameters = &parsedParams

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
		_ = utils.RenderJSONBytes(createResp.JSON200)
	}
	return nil
}

func createTestScriptFromFiles(dryRun bool) (*backplaneApi.CreateTestScriptRunJSONRequestBody, error) {
	// Read the yaml file from cwd
	yamlFile, err := os.ReadFile("metadata.yaml")
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
	fileBody, err := os.ReadFile(scriptMeta.File)

	fileBodyStr := string(fileBody)

	// if something like bin/bash or bin/sh at start, read
	if err != nil {
		logger.Errorf("unable to read file %s, make sure this file exists", scriptMeta.File)
		return nil, err
	}

	fileBodyStr, err = inlineLibrarySourceFiles(fileBodyStr, scriptMeta.File)
	if err != nil {
		return nil, err
	}

	// Base64 encode the body
	scriptBodyEncoded := base64.StdEncoding.EncodeToString([]byte(fileBodyStr))

	return &backplaneApi.CreateTestScriptRunJSONRequestBody{
		ScriptBody:     scriptBodyEncoded,
		ScriptMetadata: scriptMeta,
		DryRun:         &dryRun,
	}, nil
}

// For a managed script example.sh:
// ---
// #!/bin/bash
// source /managed-scripts/libs/lib.sh
//
// echo_foo "Hello"
// ---
//
// And function /managed-scripts/libs/lib.sh
// ---
// #!/bin/bash
//
//	function echo_foo () {
//		echo $1
//	}
//
// ---
//
// Inline into function before source definition of example.sh
// #!/bin/bash
// base64 -d <<< (based64 encoded lib.sh) > ./lib.sh
// source ./lib.sh
//
// echo_foo "Hello"
func inlineLibrarySourceFiles(script string, scriptPath string) (string, error) {
	re, err := regexp.Compile("source /managed-scripts/(.*)\n")
	if err != nil {
		return "", err
	}

	match := re.FindString(script)

	if match == "" {
		return script, nil
	}

	// i.e. /lib/foo.bash
	libraryPath := re.FindStringSubmatch(script)[1]

	// Assuming the script is inside the managed scripts directory
	scriptDir := filepath.Dir(scriptPath)

	getManagedScriptsDir := GetGitRepoPath
	getManagedScriptsDir.Dir = scriptDir

	var out bytes.Buffer
	getManagedScriptsDir.Stdout = &out

	if err = getManagedScriptsDir.Run(); err != nil {
		return "", err
	}

	managedScriptsDir := strings.TrimSpace(out.String())

	fileBody, err := os.ReadFile(managedScriptsDir + "/scripts/" + libraryPath)
	if err != nil {
		return "", err
	}
	libraryEncoded := base64.StdEncoding.EncodeToString([]byte(fileBody))

	inlinedFunction := "base64 -d <<< " + libraryEncoded + " > ./lib.sh\nsource ./lib.sh\n"

	inlinedScript := strings.Replace(script, match, inlinedFunction, 1)

	return inlinedScript, err
}
