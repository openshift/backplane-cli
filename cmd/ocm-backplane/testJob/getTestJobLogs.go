/*
Copyright © 2021 Red Hat, Inc

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package testJob

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"

	bpClient "github.com/openshift/backplane-api/pkg/client"
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

	backplaneHost, err := utils.DefaultOCMInterface.GetBackplaneURL()
	if err != nil {
		return err
	}
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
	version := "v2"
	resp, err := client.GetTestScriptRunLogs(context.TODO(), clusterID, testId, &bpClient.GetTestScriptRunLogsParams{Version: &version, Follow: &logFlag})

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
	cmd.PersistentFlags().BoolP("follow", "f", false, "Specify if logs should be streamed")
	return nil
}
