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
package managedJob

import (
	"context"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	BackplaneApi "github.com/openshift/backplane-api/pkg/client"
	"github.com/openshift/backplane-cli/pkg/utils"
)

func newGetManagedJobCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "get [job name]",
		Aliases:      []string{"ls", "list"},
		Short:        "Get a managedjob or a list of managedjobs if job name not specified",
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// ======== Parsing Flags ========
			urlFlag, err := cmd.Flags().GetString("url")
			if err != nil {
				return err
			}

			clusterKey, err := cmd.Flags().GetString("cluster-id")
			if err != nil {
				return err
			}

			rawFlag, err := cmd.Flags().GetBool("raw")
			if err != nil {
				return err
			}

			// ======== Parsing Args ========
			managedJobNameArg := ""
			if len(args) > 0 {
				managedJobNameArg = args[0]
			}

			// ======== Initialize backplaneURL ========
			bpCluster, err := utils.GetBackplaneCluster(clusterKey)
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

			// ======== Call Endpoint ========
			var jobs = make([]*BackplaneApi.Job, 0)
			if managedJobNameArg != "" {
				// Get single job
				resp, err := client.GetRun(context.TODO(), clusterID, managedJobNameArg)
				if err != nil {
					return err
				}

				if resp.StatusCode != http.StatusOK {
					return utils.TryPrintAPIError(resp, rawFlag)
				}

				jobResp, err := BackplaneApi.ParseGetRunResponse(resp)

				if err != nil {
					return fmt.Errorf("unable to parse response body from backplane: \n Status Code: %d", resp.StatusCode)
				}

				jobs = append(jobs, jobResp.JSON200)
			} else {
				resp, err := client.GetAllJobs(context.TODO(), clusterID)
				if err != nil {
					return err
				}

				if err != nil {
					return err
				}

				if resp.StatusCode != http.StatusOK {
					return utils.TryPrintAPIError(resp, rawFlag)
				}

				jobResp, err := BackplaneApi.ParseGetAllJobsResponse(resp)

				if err != nil {
					return fmt.Errorf("unable to parse response body from backplane: \n Status Code: %d", resp.StatusCode)
				}

				for _, j := range *jobResp.JSON200 {
					job := j
					jobs = append(jobs, &job)
				}
			}

			// ======== Render Results ========
			headings := []string{"jobid", "status", "namespace", "start", "script"}
			rows := make([][]string, 0)
			for _, s := range jobs {
				rows = append(rows, []string{*s.JobId, string(*s.JobStatus.Status), *s.JobStatus.Namespace, s.JobStatus.Start.String(), *s.JobStatus.Script.CanonicalName})
			}

			utils.RenderTable(headings, rows)

			return nil
		},
	}

	return cmd
}
