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
package script

import (
	"context"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	bpclient "github.com/openshift/backplane-api/pkg/client"
	"github.com/openshift/backplane-cli/pkg/utils"
)

func newListScriptCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "list",
		Aliases:      []string{"ls", "get"},
		Short:        "List available backplane scripts",
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
			// ======== Initialize backplaneURL ========
			backplaneHost := urlFlag
			if backplaneHost == "" {
				bpCluster, err := utils.GetBackplaneCluster(clusterKey, "")
				if err != nil {
					return err
				}
				backplaneHost = bpCluster.BackplaneHost
			}

			client, err := utils.DefaultClientUtils.MakeRawBackplaneAPIClient(backplaneHost)
			if err != nil {
				return err
			}

			// ======== Call Endpoint ========
			resp, err := client.GetScripts(context.TODO(), &bpclient.GetScriptsParams{})

			if err != nil {
				return err
			}

			if resp.StatusCode != http.StatusOK {
				return utils.TryPrintAPIError(resp, rawFlag)
			}

			// ======== Render Table ========
			listResp, err := bpclient.ParseGetScriptsResponse(resp)

			if err != nil {
				return fmt.Errorf("unable to parse response body from backplane: Status Code: %d", resp.StatusCode)
			}

			scriptList := *(*[]bpclient.Script)(listResp.JSON200) 
			if (len(scriptList) == 0) {
				return fmt.Errorf("no scripts found")
			}

			headings := []string{"NAME", "DESCRIPTION"}
			rows := make([][]string, 0)
			for _, s := range scriptList {
				rows = append(rows, []string{*s.CanonicalName, *s.Description})
			}

			utils.RenderTabbedTable(headings, rows)

			return nil
		},
	}
	return cmd
}
