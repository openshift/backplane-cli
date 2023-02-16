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
package script

import (
	"github.com/spf13/cobra"
)

func NewScriptCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "script",
		Aliases:      []string{"scripts"},
		Short:        "Represents a backplane script resource.",
		SilenceUsage: true,
	}

	// url flag
	// Denotes backplane url
	// If this flag is empty, its value will be populated by --cluster-id flag supplied by user. cluster-id flag will be used to find corresponding hive-shard and composing backplane url.
	cmd.PersistentFlags().String(
		"url",
		"",
		"Specify backplane url. Default: The corresponding hive shard of the target cluster.",
	)

	// cluster-id Flag
	cmd.PersistentFlags().StringP(
		"cluster-id",
		"c",
		"",
		"Cluster ID could be cluster name, id or external-id")

	// raw Flag
	cmd.PersistentFlags().Bool("raw", false, "Prints the raw response returned by the backplane API")

	cmd.AddCommand(newListScriptCmd())
	cmd.AddCommand(newDescribeScriptCmd())
	return cmd
}
