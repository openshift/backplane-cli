package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/openshift-online/ocm-cli/pkg/ocm"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	BackplaneApi "github.com/openshift/backplane-api/pkg/client"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	bpCredentials "github.com/openshift/backplane-cli/pkg/credentials"
	"github.com/openshift/backplane-cli/pkg/utils"
	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
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

	// Initialize backplane configuration
	backplaneConfiguration, err := config.GetBackplaneConfiguration()
	if err != nil {
		return fmt.Errorf("unable to build backplane configuration: %w", err)
	}

	// ============Get Backplane URl ==========================
	if credentialArgs.backplaneURL != "" { // Overwrite if parameter is set
		backplaneConfiguration.URL = credentialArgs.backplaneURL
	}

	if backplaneConfiguration.URL == "" {
		return errors.New("empty backplane url - check your backplane-cli configuration")
	}
	logger.Infof("Using backplane URL: %s\n", backplaneConfiguration.URL)

	// Initialize OCM sdk
	ocmSdk, err := ocm.NewConnection().Build()
	if err != nil {
		return fmt.Errorf("unable to build ocm sdk: %w", err)
	}
	defer ocmSdk.Close()

	// ======== Call Endpoint ==================================
	logger.Debugln("Getting Cloud Credentials")

	queryConfig := &CloudQueryConfig{OcmConnection: ocmSdk, BackplaneConfiguration: backplaneConfiguration}

	credsResp, err := getCloudCredentials(queryConfig, cluster)
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

// getCloudCredentials returns Cloud Credentials Response
func getCloudCredentials(queryConfig *CloudQueryConfig, cluster *cmv1.Cluster) (bpCredentials.Response, error) {
	ocmToken, _, err := queryConfig.OcmConnection.Tokens()
	if err != nil {
		return nil, fmt.Errorf("unable to get token for ocm connection")
	}

	isolatedBackplane, err := isIsolatedBackplaneAccess(cluster, queryConfig.OcmConnection)
	if err != nil {
		logger.Infof("failed to determine if the cluster is using isolated backplane access: %v", err)
		logger.Infof("for more information, try ocm get /api/clusters_mgmt/v1/clusters/%s/sts_support_jump_role", cluster.ID())
		logger.Infof("attempting to fallback to %s", OldFlowSupportRole)
	}

	if isolatedBackplane {
		logger.Debugf("cluster is using isolated backplane")
		targetCredentials, err := getIsolatedCredentials(cluster.ID(), queryConfig, &ocmToken)
		if err != nil {
			// TODO: This fallback should be removed in the future
			// TODO: when we are more confident in our ability to access clusters using the isolated flow
			logger.Infof("failed to assume role with isolated backplane flow: %v", err)
			logger.Infof("attempting to fallback to %s", OldFlowSupportRole)
			return getCloudCredentialsFromBackplaneAPI(queryConfig, ocmToken, cluster)
		}

		return &bpCredentials.AWSCredentialsResponse{
			AccessKeyID:     targetCredentials.AccessKeyID,
			SecretAccessKey: targetCredentials.SecretAccessKey,
			SessionToken:    targetCredentials.SessionToken,
			Expiration:      targetCredentials.Expires.String(),
			Region:          cluster.Region().ID(),
		}, nil
	}

	return getCloudCredentialsFromBackplaneAPI(queryConfig, ocmToken, cluster)
}

func getCloudCredentialsFromBackplaneAPI(queryConfig *CloudQueryConfig, ocmToken string, cluster *cmv1.Cluster) (bpCredentials.Response, error) {
	client, err := utils.DefaultClientUtils.GetBackplaneClient(queryConfig.BackplaneConfiguration.URL, ocmToken, queryConfig.BackplaneConfiguration.ProxyURL)
	if err != nil {
		return nil, err
	}

	resp, err := client.GetCloudCredentials(context.TODO(), cluster.ID())
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, utils.TryPrintAPIError(resp, false)
	}

	logger.Debugln("Parsing response")

	credsResp, err := BackplaneApi.ParseGetCloudCredentialsResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("unable to parse response body from backplane:\n  Status Code: %d : err: %v", resp.StatusCode, err)
	}

	switch cluster.CloudProvider().ID() {
	case "aws":
		cliResp := &bpCredentials.AWSCredentialsResponse{}
		if err := json.Unmarshal([]byte(*credsResp.JSON200.Credentials), cliResp); err != nil {
			return nil, fmt.Errorf("unable to unmarshal AWS credentials response from backplane %s: %w", *credsResp.JSON200.Credentials, err)
		}
		cliResp.Region = cluster.Region().ID()
		return cliResp, nil
	case "gcp":
		cliResp := &bpCredentials.GCPCredentialsResponse{}
		if err := json.Unmarshal([]byte(*credsResp.JSON200.Credentials), cliResp); err != nil {
			return nil, fmt.Errorf("unable to unmarshal GCP credentials response from backplane %s: %w", *credsResp.JSON200.Credentials, err)
		}
		return cliResp, nil
	default:
		return nil, fmt.Errorf("unsupported cloud provider: %s", cluster.CloudProvider().ID())
	}
}

// GetAWSV2ConfigCustom allows consumers to get an aws-sdk-go-v2 Config to programmatically access the AWS API
func GetAWSV2Config(queryConfig *CloudQueryConfig, cluster *cmv1.Cluster) (aws.Config, error) {
	if cluster.CloudProvider().ID() != "aws" {
		return aws.Config{}, fmt.Errorf("only supported for the aws cloud provider, this cluster has: %s", cluster.CloudProvider().ID())
	}
	creds, err := getCloudCredentials(queryConfig, cluster)
	if err != nil {
		return aws.Config{}, err
	}

	awsCreds, ok := creds.(*bpCredentials.AWSCredentialsResponse)
	if !ok {
		return aws.Config{}, errors.New("unexpected error: failed to convert backplane creds to AWSCredentialsResponse")
	}

	return awsCreds.AWSV2Config()
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
