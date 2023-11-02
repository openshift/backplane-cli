package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	bpCredentials "github.com/openshift/backplane-cli/pkg/credentials"
	"net/http"

	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	BackplaneApi "github.com/openshift/backplane-api/pkg/client"

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

	clusterID, clusterName, err := utils.DefaultOCMInterface.GetTargetCluster(clusterKey)
	if err != nil {
		return err
	}

	cluster, err := utils.DefaultOCMInterface.GetClusterInfoByID(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster info for %s: %w", clusterID, err)
	}

	cloudProvider := utils.DefaultClusterUtils.GetCloudProvider(cluster)

	logger.WithFields(logger.Fields{
		"ID":   clusterID,
		"Name": clusterName}).Infoln("Target cluster")

	// ============Get Backplane URl ==========================
	bpURL := ""
	if credentialArgs.backplaneURL != "" {
		bpURL = credentialArgs.backplaneURL
	} else {
		// Get Backplane configuration
		bpConfig, err := GetBackplaneConfiguration()
		if err != nil {
			return fmt.Errorf("can't find backplane url: %w", err)
		}

		if bpConfig.URL == "" {
			return errors.New("empty backplane url - check your backplane-cli configuration")
		}
		bpURL = bpConfig.URL
		logger.Infof("Using backplane URL: %s\n", bpURL)
	}

	// ======== Call Endpoint ==================================
	logger.Debugln("Getting Cloud Credentials")

	var output string
	isolatedBackplane, err := isIsolatedBackplaneAccess(cluster)
	if err != nil {
		return fmt.Errorf("failed to determine if cluster is using isolated backplane access: %w", err)
	}
	if isolatedBackplane {
		targetCredentials, err := getIsolatedCredentials(clusterID)
		if err != nil {
			return fmt.Errorf("failed to get cloud credentials for cluster %v: %w", clusterID, err)
		}

		bpCreds := &bpCredentials.AWSCredentialsResponse{
			AccessKeyID:     targetCredentials.AccessKeyID,
			SecretAccessKey: targetCredentials.SecretAccessKey,
			SessionToken:    targetCredentials.SessionToken,
			Expiration:      targetCredentials.Expires.String(),
		}
		if region, ok := cluster.GetRegion(); ok {
			bpCreds.Region = region.ID()
		}

		output, err = renderCloudCredentials(credentialArgs.output, bpCreds)
		if err != nil {
			return fmt.Errorf("failed to render credentials: %w", err)
		}
	} else {
		credsResp, err := getCloudCredential(bpURL, clusterID)
		if err != nil {
			return fmt.Errorf("failed to get cloud credentials for cluster %v: %w", clusterID, err)
		}
		output, err = renderCredentials(credsResp.JSON200.Credentials, credsResp.JSON200.Region, cloudProvider)
		if err != nil {
			return err
		}
	}

	fmt.Println(output)
	return nil
}

// getCloudCredential returns Cloud Credentials Response
func getCloudCredential(backplaneURL string, clusterID string) (*BackplaneApi.GetCloudCredentialsResponse, error) {
	client, err := utils.DefaultClientUtils.GetBackplaneClient(backplaneURL)
	if err != nil {
		return nil, err
	}

	resp, err := client.GetCloudCredentials(context.TODO(), clusterID)

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

	if len(credsResp.Body) == 0 {
		return nil, fmt.Errorf("empty response from backplane")
	}
	return credsResp, nil
}

func renderCredentials(credentials *string, region *string, cloudProvider string) (string, error) {
	switch cloudProvider {
	case "aws":
		cliResp := &bpCredentials.AWSCredentialsResponse{}
		if err := json.Unmarshal([]byte(*credentials), cliResp); err != nil {
			return "", fmt.Errorf("unable to unmarshal AWS credentials response from backplane %s: %w", *credentials, err)
		}
		cliResp.Region = aws.ToString(region)
		creds, err := renderCloudCredentials(credentialArgs.output, cliResp)
		if err != nil {
			return "", err
		}
		return creds, nil
	case "gcp":
		cliResp := &bpCredentials.GCPCredentialsResponse{}
		if err := json.Unmarshal([]byte(*credentials), cliResp); err != nil {
			return "", fmt.Errorf("unable to unmarshal GCP credentials response from backplane %s: %w", *credentials, err)
		}
		creds, err := renderCloudCredentials(credentialArgs.output, cliResp)
		if err != nil {
			return "", err
		}
		return creds, nil
	default:
		return "", fmt.Errorf("unsupported cloud provider: %s", cloudProvider)
	}
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
