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
	"net/url"

	"github.com/spf13/cobra"

	bpclient "github.com/openshift/backplane-api/pkg/client"

	"github.com/openshift/backplane-cli/pkg/backplaneapi"
	"github.com/openshift/backplane-cli/pkg/ocm"
	"github.com/openshift/backplane-cli/pkg/utils"
)

func newDescribeScriptCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "describe",
		Short:        "Describe the given script",
		Args:         cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
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
			// ======== Initialize backplaneURL == ========
			backplaneHost := urlFlag
			if backplaneHost != "" {
				// Validate urlFlag if it's provided
				parsedURL, parseErr := url.ParseRequestURI(urlFlag)
				if parseErr != nil {
					return fmt.Errorf("invalid --url: %v", parseErr)
				}
				if parsedURL.Scheme != "https" {
					return fmt.Errorf("invalid --url '%s': scheme must be https", urlFlag)
				}
			} else {
				// If urlFlag is empty, get it from cluster config
				bpCluster, err := utils.DefaultClusterUtils.GetBackplaneCluster(clusterKey, urlFlag)
				if err != nil {
					return err
				}
				backplaneHost = bpCluster.BackplaneHost
			}

			client, err := backplaneapi.DefaultClientUtils.MakeRawBackplaneAPIClient(backplaneHost)
			if err != nil {
				return err
			}

			// ======== Initialize cluster ID from config ========
			if clusterKey == "" {
				configCluster, err := utils.DefaultClusterUtils.GetBackplaneClusterFromConfig()
				if err != nil {
					return err
				}
				clusterKey = configCluster.ClusterID
			}

			// ======== Transform clusterKey to clusterID (clusterKey can be name, ID external ID) ========
			clusterID, _, err := ocm.DefaultOCMInterface.GetTargetCluster(clusterKey)
			if err != nil {
				return err
			}

			// ======== Call Endpoint ========
			resp, err := client.GetScriptsByCluster(context.TODO(), clusterID, &bpclient.GetScriptsByClusterParams{Scriptname: &args[0]})

			if err != nil {
				return err
			}

			if resp.StatusCode != http.StatusOK {
				return utils.TryPrintAPIError(resp, rawFlag)
			}

			// ======== Print script info ========
			describeResp, err := bpclient.ParseGetScriptsByClusterResponse(resp)

			if err != nil {
				return fmt.Errorf("unable to parse response body from backplane: Status Code: %d", resp.StatusCode)
			}

			scripts := *(*[]bpclient.Script)(describeResp.JSON200)
			if len(scripts) == 0 {
				return fmt.Errorf("server misbehave: returned %d without result", resp.StatusCode)
			}
			script := scripts[0]

			// print basic info
			fmt.Printf(
				"CanonicalName: %s\n"+
					"Author:        %s\n"+
					"Description:   %s\n"+
					"AllowedGroups: %s\n"+
					"Language:      %s\n"+
					"Permalink:     %s\n",
				*script.CanonicalName,
				*script.Author,
				*script.Description,
				*script.AllowedGroups,
				*script.Language,
				*script.Permalink,
			)

			if script.Envs == nil {
				return nil
			}
			params := *script.Envs
			// required parameters
			fmt.Println("Required Parameters:")
			printParams(params, false)
			// optional parameters
			fmt.Println("Optional Parameters:")
			printParams(params, true)
			return nil
		},
	}
	return cmd
}

func printParams(params []bpclient.EnvDecl, optional bool) {
	for _, p := range params {
		if *p.Optional == optional {
			fmt.Printf(
				"- Key:         %s\n"+
					"  Description: %s\n",
				*p.Key,
				*p.Description,
			)
		}
	}
}