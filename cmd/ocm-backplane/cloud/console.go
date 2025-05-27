package cloud

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"

	"github.com/openshift/backplane-cli/pkg/ocm"

	"github.com/pkg/browser"
	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

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

	clusterID, clusterName, err := ocm.DefaultOCMInterface.GetTargetCluster(clusterKey)
	if err != nil {
		return err
	}

	cluster, err := ocm.DefaultOCMInterface.GetClusterInfoByID(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster info for %s: %w", clusterID, err)
	}

	logger.WithFields(logger.Fields{
		"ID":   clusterID,
		"Name": clusterName}).Infoln("Target cluster")

	// Initialize backplane configuration
	backplaneConfiguration, err := config.GetBackplaneConfiguration()
	if err != nil {
		return fmt.Errorf("unable to build backplane configuration: %w", err)
	}

	// ============Get Backplane URl ==========================
	if consoleArgs.backplaneURL != "" { // Overwrite if parameter is set
		parsedURL, parseErr := url.ParseRequestURI(consoleArgs.backplaneURL)
		if parseErr != nil {
			return fmt.Errorf("invalid --url: %v", parseErr)
		}
		if parsedURL.Scheme != "https" {
			return fmt.Errorf("invalid --url '%s': scheme must be https", consoleArgs.backplaneURL)
		}
		backplaneConfiguration.URL = consoleArgs.backplaneURL
		logger.Infof("Using backplane URL: %s\n", backplaneConfiguration.URL)
	} else {
		// Log the URL from config if custom one isn't provided
		logger.Infof("Using backplane URL: %s\n", backplaneConfiguration.URL)
	}

	// Initialize OCM connection
	ocmConnection, err := ocm.DefaultOCMInterface.SetupOCMConnection()
	if err != nil {
		return fmt.Errorf("failed to create OCM connection: %w", err)
	}
	defer ocmConnection.Close()

	// Initialize query config

	queryConfig := &QueryConfig{OcmConnection: ocmConnection, BackplaneConfiguration: backplaneConfiguration, Cluster: cluster}

	// ======== Get cloud console from backplane API ============
	consoleResponse, err := queryConfig.GetCloudConsole()

	// Declare helperMsg
	helperMsg := "\n\033[1mNOTE: To troubleshoot the connectivity issues, please run `ocm-backplane health-check`\033[0m\n\n"

	if err != nil {
		// Check API connection with configured proxy
		if connErr := backplaneConfiguration.CheckAPIConnection(); connErr != nil {
			logger.Error("Cannot connect to backplane API URL, check if you need to use a proxy/VPN to access backplane:")
			logger.Errorf("Error: %v.\n%s", connErr, helperMsg)
		}

		return fmt.Errorf("failed to get cloud console for cluster %v: %w", clusterID, err)
	}

	err = renderCloudConsole(consoleResponse)
	if err != nil {
		return fmt.Errorf("failed to render cloud console: %w", err)
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