/*
Copyright © 2021 Red Hat, Inc.

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

package status

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/openshift/backplane-cli/pkg/utils"
)

var StatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current backplane login info",
	Long: `It will read the effecitve cluster id from the kubeconfig,
	and print the essential info.
`,
	Args:         cobra.ExactArgs(0),
	RunE:         runStatus,
	SilenceUsage: true,
}

func runStatus(cmd *cobra.Command, argv []string) error {

	clusterInfo, err := utils.DefaultClusterUtils.GetBackplaneClusterFromConfig()
	if err != nil {
		return err
	}

	clusterV1, err := utils.DefaultOCMInterface.GetClusterInfoByID(clusterInfo.ClusterID)
	if err != nil {
		return err
	}

	clusterName := clusterV1.Name()
	basedomain := clusterV1.DNS().BaseDomain()

	fmt.Printf(
		"Cluster ID:         %s\n"+
			"Cluster Name:       %s\n"+
			"Cluster Basedomain: %s\n"+
			"Backplane Server:   %s\n",
		clusterInfo.ClusterID,
		clusterName,
		basedomain,
		clusterInfo.BackplaneHost,
	)

	return nil
}
