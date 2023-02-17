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
	"gopkg.in/AlecAivazis/survey.v1"

	"github.com/openshift/backplane-cli/pkg/utils"
)

func newDeleteManagedJobCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "delete <job name>",
		Aliases:      []string{"del"},
		Short:        "Delete a managed job",
		Args:         cobra.ExactArgs(1),
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

			// assume-yes flag
			yesFlag, err := cmd.Flags().GetBool("yes")
			if err != nil {
				return err
			}

			// ======== Parsing Args ========
			if len(args) < 1 {
				return fmt.Errorf("missing managedjob name as an argument")
			}
			managedJobName := args[0]

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

			// ======== Warn User ========
			if !yesFlag {
				confirm := false
				prompt := &survey.Confirm{
					Message: "Deleting the job will also delete the logs.\nDo you want to continue?",
				}
				err := survey.AskOne(prompt, &confirm, nil)
				if err != nil {
					return err
				}
				if !confirm {
					fmt.Printf("Aborted.\n")
					return nil
				}
			}

			client, err := utils.DefaultClientUtils.MakeRawBackplaneAPIClient(backplaneHost)
			if err != nil {
				return err
			}

			// ======== Call Endpoint ========
			resp, err := client.DeleteJob(context.TODO(), clusterID, managedJobName)

			// ======== Render Results ========
			if err != nil {
				return err
			}

			if resp.StatusCode != http.StatusOK {
				return utils.TryPrintAPIError(resp, false)
			}

			fmt.Printf("Deleted Job ID: %s\n", managedJobName)
			return nil
		},
	}
	cmd.Flags().BoolP("yes", "y", false, "Answer yes to all prompts")
	return cmd
}
