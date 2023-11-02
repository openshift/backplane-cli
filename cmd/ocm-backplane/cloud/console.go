package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/openshift/backplane-cli/pkg/awsutil"
	"net/http"
	"os"
	"strconv"

	"github.com/pkg/browser"
	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	BackplaneApi "github.com/openshift/backplane-api/pkg/client"

	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/utils"
)

var consoleArgs struct {
	browser      bool
	backplaneURL string
	output       string
}

type ConsoleResponse struct {
	ConsoleLink string `json:"ConsoleLink" yaml:"ConsoleLink"`
}

var consoleStrFmt = `Console Link:
  Link: %s`

func (r *ConsoleResponse) String() string {
	return fmt.Sprintf(consoleStrFmt, r.ConsoleLink)
}

// EnvBrowserDefault environment variable that indicates if open by browser is set as default
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
		"URL of backplane API",
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

	utils.CheckBackplaneVersion(cmd)

	err = validateParams(argv)

	if err != nil {
		return err
	}

	// ========= Get The cluster ID =============================
	if len(argv) == 1 {
		// if explicitly one cluster key given, use it to log in.
		clusterKey = argv[0]
		logger.WithField("Search Key", clusterKey).Debugln("Finding target cluster")
	} else if len(argv) == 0 {
		// if no args given, try to log into the cluster that the user is logged into
		clusterInfo, err := utils.DefaultClusterUtils.GetBackplaneClusterFromConfig()
		if err != nil {
			return err
		}
		clusterKey = clusterInfo.ClusterID
	}

	clusterID, clusterName, err := utils.DefaultOCMInterface.GetTargetCluster(clusterKey)
	if err != nil {
		return err
	}

	cluster, err := utils.DefaultOCMInterface.GetClusterInfoByID(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster info for %s: %w", clusterID, err)
	}

	logger.WithFields(logger.Fields{
		"ID":   clusterID,
		"Name": clusterName}).Infoln("Target cluster")

	// ============Get Backplane URl ==========================
	bpURL := ""
	if consoleArgs.backplaneURL != "" {
		bpURL = consoleArgs.backplaneURL
	} else {
		// Get Backplane configuration
		bpConfig, err := config.GetBackplaneConfiguration()
		if err != nil {
			return fmt.Errorf("can't find backplane url: %w", err)
		}

		if bpConfig.URL == "" {
			return errors.New("empty backplane url - check your backplane-cli configuration")
		}
		bpURL = bpConfig.URL
		logger.Infof("Using backplane URL: %s\n", bpURL)
	}

	// ======== Get cloud console from backplane API ============

	var consoleResponse *ConsoleResponse
	isolatedBackplane, err := isIsolatedBackplaneAccess(cluster)
	if err != nil {
		return fmt.Errorf("failed to determine if cluster is using isolated backplane access: %w", err)
	}
	if isolatedBackplane {
		targetCredentials, err := getIsolatedCredentials(clusterID)
		if err != nil {
			// itn-2023-00143 handle case where customer's org is on the isolated flow,
			// but they have not yet migrated their account roles
			fmt.Println("Cluster's org is using new flow but cluster has not migrated to new account roles. Trying old flow...")
			consoleResponse, err = getCloudConsole(bpURL, clusterID)
			if err != nil {
				return err
			}
		} else {
			resp, err := awsutil.GetSigninToken(targetCredentials, cluster.Region().ID())
			if err != nil {
				return fmt.Errorf("failed to get signin token: %w", err)
			}

			signinFederationURL, err := awsutil.GetConsoleURL(resp.SigninToken, cluster.Region().ID())
			if err != nil {
				return fmt.Errorf("failed to generate console url: %w", err)
			}
			consoleResponse = &ConsoleResponse{ConsoleLink: signinFederationURL.String()}
		}
	} else {
		consoleResponse, err = getCloudConsole(bpURL, clusterID)
		if err != nil {
			return err
		}
	}

	err = renderCloudConsole(consoleResponse)
	if err != nil {
		return err
	}
	return nil
}

func validateParams(argv []string) (err error) {
	// Check if env variable 'BACKPLANE_DEFAULT_OPEN_BROWSER' is set
	if env, ok := os.LookupEnv(EnvBrowserDefault); ok {
		// if set, try to parse it as a bool and pass it into consoleArgs.browser
		consoleArgs.browser, err = strconv.ParseBool(env)
		if err != nil {
			return fmt.Errorf("unable to parse boolean value from environment variable %s", EnvBrowserDefault)
		}
	}
	if len(argv) > 1 {
		return fmt.Errorf("expected exactly one cluster")
	}
	return nil
}

// getCloudConsole returns console response calling to public Backplane API
func getCloudConsole(backplaneURL string, clusterID string) (*ConsoleResponse, error) {
	logger.Debugln("Getting Cloud Console")
	client, err := utils.DefaultClientUtils.GetBackplaneClient(backplaneURL)
	if err != nil {
		return nil, err
	}
	resp, err := client.GetCloudConsole(context.TODO(), clusterID)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, utils.TryPrintAPIError(resp, false)
	}

	credsResp, err := BackplaneApi.ParseGetCloudConsoleResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("unable to parse response body from backplane:\n  Status Code: %d", resp.StatusCode)
	}

	if len(credsResp.Body) == 0 {
		return nil, fmt.Errorf("empty response from backplane")
	}

	cliResp := &ConsoleResponse{}
	cliResp.ConsoleLink = *credsResp.JSON200.ConsoleLink

	return cliResp, nil
}

// renderCloudConsole output the data based output type
func renderCloudConsole(response *ConsoleResponse) error {

	if consoleArgs.browser {
		if err := browser.OpenURL(response.ConsoleLink); err != nil {
			logger.Warnf("failed opening browser: %s", err)
		} else {
			return nil
		}
	}

	switch consoleArgs.output {
	case "yaml":
		yamlBytes, err := yaml.Marshal(response)
		if err != nil {
			return err
		}
		fmt.Println("---")
		fmt.Println(string(yamlBytes))
		return nil
	case "json":
		jsonBytes, err := json.Marshal(response)
		if err != nil {
			return err
		}
		fmt.Println(string(jsonBytes))
		return nil
	default:
		fmt.Println(response)
	}

	return nil
}
