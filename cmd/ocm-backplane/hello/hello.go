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

package hello

import (
	"fmt"
	"github.com/spf13/cobra"
)

// HelloCmd represents the hello command
var HelloCmd = &cobra.Command{
	Use:   "hello",
	Short: "Print hello",
	Long: ` The Hello cmd helps to print in hello world for its users` ,

	Run: func(cmd *cobra.Command, args []string) {
		output := runHello(cmd,args)
		fmt.Println(output)
	},
	SilenceUsage: true,
}

func runHello(cmd *cobra.Command,_[]string)bool{
	fmt.Println("Hello world")
	return true
}
