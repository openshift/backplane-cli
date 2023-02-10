/*
Copyright © 2020 Red Hat, Inc.

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

package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/go-yaml/yaml"
	"github.com/pkg/browser"
	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	BackplaneApi "github.com/openshift/backplane-cli/pkg/client"
	"github.com/openshift/backplane-cli/pkg/utils"
)

var consoleArgs struct {
	browser      bool
	backplaneURL string
	output       string
}

type AWSConsoleResponse struct {
	ConsoleLink string `json:"ConsoleLink" yaml:"ConsoleLink"`
}

var consoleStrFmt string = `%s
Console Link:
  Link: %s`

var consoleBanner string = `NOTE: If you are going to check Route53 or IAM related CloudTrail events, ` +
	`please switch to us-east-1 regardless the cluster region.
`

func (r *AWSConsoleResponse) String() string {
	return fmt.Sprintf(consoleStrFmt, consoleBanner, r.ConsoleLink)
}

// Environment variable that indicates if open by browser is set as default
const EnvBrowserDefault = "BACKPLANE_DEFAULT_OPEN_BROWSER"

// ConsoleCmd represents the cloud credentials command
var ConsoleCmd = &cobra.Command{
	Use:   "console [CLUSTERID|EXTERNAL_ID|CLUSTER_NAME|CLUSTER_NAME_SEARCH]",
	Short: "Requests a link to cluster's cloud provider's console",
	Long: `Requests a link that utilizes temporary cloud credentials for the cluster's cloud provider's web console.
	This allows us to be able to perform operations such as debugging an issue, troubleshooting a customer
	misconfiguration, or directly access the underlying cloud infrastructure. If no cluster identifier is provided, the
	currently logged in cluster will be used.`,
	Example:      " backplane cloud console\n backplane cloud console <id>\n backplane cloud console %test%\n backplane cloud console <external_id>",
	Args:         cobra.RangeArgs(0, 1),
	Aliases:      []string{"link", "web"},
	RunE:         runConsole,
	SilenceUsage: true,
}

func init() {
	flags := ConsoleCmd.Flags()
	flags.BoolVarP(
		&consoleArgs.browser,
		"browser",
		"b",
		false,
		fmt.Sprintf("Open a browser after the console container starts. Can also be set via the environment variable '%s'", EnvBrowserDefault),
	)
	flags.StringVar(
		&consoleArgs.backplaneURL,
		"url",
		"",
		"Specify backplane url. Default: The corresponding hive shard of the target cluster.",
	)
	flags.StringVarP(
		&consoleArgs.output,
		"output",
		"o",
		"text",
		"Format the output of the console response.",
	)
}

func runConsole(cmd *cobra.Command, argv []string) (err error) {
	var clusterKey string

	// Check if env variable 'BACKPLANE_DEFAULT_OPEN_BROWSER' is set
	if env, ok := os.LookupEnv(EnvBrowserDefault); ok {
		// if set, try to parse it as a bool and pass it into consoleArgs.browser
		consoleArgs.browser, err = strconv.ParseBool(env)
		if err != nil {
			return fmt.Errorf("unable to parse boolean value from environment variable %s", EnvBrowserDefault)
		}
	}
	if len(argv) == 1 {
		// if explicitly one cluster key given, use it to log in.
		clusterKey = argv[0]
		logger.WithField("Search Key", clusterKey).Debugln("Finding target cluster")
	} else if len(argv) == 0 {
		// if no args given, try to log into the cluster that the user is logged into
		clusterInfo, err := utils.GetBackplaneClusterFromConfig()
		if err != nil {
			return err
		}
		clusterKey = clusterInfo.ClusterID
	} else {
		return fmt.Errorf("expected exactly one cluster")
	}

	clusterId, clusterName, err := utils.DefaultOCMInterface.GetTargetCluster(clusterKey)
	if err != nil {
		return err
	}

	logger.WithFields(logger.Fields{
		"ID":   clusterId,
		"Name": clusterName}).Infoln("Target cluster")

	// Lookup backplane url
	if consoleArgs.backplaneURL == "" {
		consoleArgs.backplaneURL, err = utils.DefaultOCMInterface.GetBackplaneShard(clusterId)
		if err != nil || consoleArgs.backplaneURL == "" {
			return fmt.Errorf("can't find shard url: %w", err)
		}
		logger.Infof("Using backplane URL: %s\n", consoleArgs.backplaneURL)
	}

	logger.Debugln("Finding ocm token")
	accessToken, err := utils.DefaultOCMInterface.GetOCMAccessToken()
	if err != nil {
		return err
	}
	logger.Debugln("Found OCM access token")

	logger.Debugln("Getting client")
	client, err := utils.DefaultClientUtils.MakeRawBackplaneAPIClientWithAccessToken(consoleArgs.backplaneURL, *accessToken)
	if err != nil {
		return fmt.Errorf("unable to create backplane api client: %w", err)
	}
	logger.Debugln("Got Client")

	// ======== Call Endpoint ========
	logger.Debugln("Getting Cloud Console")
	resp, err := client.GetCloudConsole(context.TODO(), clusterId)
	if err != nil {
		// trying to determine the error
		errBody := err.Error()
		if strings.Contains(errBody, "dial tcp") && strings.Contains(errBody, "i/o timeout") {
			// Likely tunnel problem
			return fmt.Errorf("unable to connect to backplane api, please check if the tunnel is running")
		}
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return utils.TryPrintAPIError(resp, false)
	}

	credsResp, err := BackplaneApi.ParseGetCloudConsoleResponse(resp)
	if err != nil {
		return fmt.Errorf("unable to parse response body from backplane:\n  Status Code: %d", resp.StatusCode)
	}

	if len(credsResp.Body) == 0 {
		return fmt.Errorf("empty response from backplane")
	}

	// ======== Render results ========

	cliResp := &AWSConsoleResponse{}
	cliResp.ConsoleLink = *credsResp.JSON200.ConsoleLink

	if consoleArgs.browser {
		if err := browser.OpenURL(cliResp.ConsoleLink); err != nil {
			logger.Warnf("failed opening browser: %s", err)
		} else {
			return nil
		}
	}

	switch consoleArgs.output {
	case "yaml":
		yamlBytes, err := yaml.Marshal(cliResp)
		if err != nil {
			return err
		}
		fmt.Println("---")
		fmt.Println(string(yamlBytes))
		return nil
	case "json":
		jsonBytes, err := json.Marshal(cliResp)
		if err != nil {
			return err
		}
		fmt.Println(string(jsonBytes))
		return nil
	default:
		fmt.Println(cliResp)
	}
	return nil
}
