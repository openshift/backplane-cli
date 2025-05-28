package cloud

import (
	"encoding/json"
	"fmt"
	"net/url"

	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	"github.com/openshift/backplane-cli/pkg/cli/config"
	bpCredentials "github.com/openshift/backplane-cli/pkg/credentials"
	"github.com/openshift/backplane-cli/pkg/ocm"
	"github.com/openshift/backplane-cli/pkg/utils"
)

var GetBackplaneClusterFromConfig = utils.DefaultClusterUtils.GetBackplaneClusterFromConfig

var credentialArgs struct {
	backplaneURL string
	output       string
}

// CredentialsCmd represents the cloud credentials command
var CredentialsCmd = &cobra.Command{
	Use:   "credentials [CLUSTERID|EXTERNAL_ID|CLUSTER_NAME|CLUSTER_NAME_SEARCH]",
	Short: "Requests a set of temporary cloud credentials for the cluster's cloud provider",
	Long: `Requests a set of temporary cloud credentials for the cluster's cloud provider. This allows us to be able to
	perform operations such as debugging an issue, troubleshooting a customer misconfiguration, or directly access the
	underlying cloud infrastructure. If no cluster identifier is provided, the currently logged in cluster will be used.`,
	Example:      " backplane cloud credentials\n backplane cloud credentials <id>\n backplane cloud credentials %test%\n backplane cloud credentials <external_id>",
	Args:         cobra.RangeArgs(0, 1),
	Aliases:      []string{"creds", "cred"},
	RunE:         runCredentials,
	SilenceUsage: true,
}

func init() {
	flags := CredentialsCmd.Flags()
	flags.StringVar(
		&credentialArgs.backplaneURL,
		"url",
		"",
		"URL of backplane API",
	)
	flags.StringVarP(
		&credentialArgs.output,
		"output",
		"o",
		"text",
		"Format the output of the credentials response. One of text|json|yaml|env",
	)
}

func runCredentials(cmd *cobra.Command, argv []string) error {
	var clusterKey string

	if len(argv) == 1 {
		// if explicitly one cluster key given, use it to log in.
		clusterKey = argv[0]
		logger.WithField("Search Key", clusterKey).Debugln("Finding target cluster")
	} else if len(argv) == 0 {
		// if no args given, try to log into the cluster that the user is logged into
		clusterInfo, err := GetBackplaneClusterFromConfig()
		if err != nil {
			return err
		}
		clusterKey = clusterInfo.ClusterID
	} else {
		return fmt.Errorf("expected exactly one cluster")
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
	if credentialArgs.backplaneURL != "" { // Overwrite if parameter is set
		parsedURL, parseErr := url.ParseRequestURI(credentialArgs.backplaneURL)
		if parseErr != nil {
			return fmt.Errorf("invalid --url: %v", parseErr)
		}
		if parsedURL.Scheme != "https" {
			return fmt.Errorf("invalid --url '%s': scheme must be https", credentialArgs.backplaneURL)
		}
		backplaneConfiguration.URL = credentialArgs.backplaneURL
	}
	logger.Infof("Using backplane URL: %s\n", backplaneConfiguration.URL)

	// Initialize OCM connection
	ocmConnection, err := ocm.DefaultOCMInterface.SetupOCMConnection()
	if err != nil {
		return fmt.Errorf("failed to create OCM connection: %w", err)
	}
	defer ocmConnection.Close()

	// ======== Call Endpoint ==================================
	logger.Debugln("Getting Cloud Credentials")

	queryConfig := &QueryConfig{OcmConnection: ocmConnection, BackplaneConfiguration: backplaneConfiguration, Cluster: cluster}

	credsResp, err := queryConfig.GetCloudCredentials()
	if err != nil {
		return fmt.Errorf("failed to get cloud credentials for cluster %v: %w", clusterID, err)
	}

	output, err := renderCloudCredentials(credentialArgs.output, credsResp)
	if err != nil {
		return fmt.Errorf("failed to render credentials: %w", err)
	}

	fmt.Println(output)
	return nil
}

// renderCloudCredentials displays the results of `ocm backplane cloud credentials` for AWS clusters
func renderCloudCredentials(outputFormat string, creds bpCredentials.Response) (string, error) {
	switch outputFormat {
	case "env":
		return creds.FmtExport(), nil
	case "yaml":
		yamlBytes, err := yaml.Marshal(creds)
		if err != nil {
			return "", err
		}
		return string(yamlBytes), nil
	case "json":
		jsonBytes, err := json.Marshal(creds)
		if err != nil {
			return "", err
		}

		return string(jsonBytes), nil
	default:
		return creds.String(), nil
	}
}