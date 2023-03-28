package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-yaml/yaml"
	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	BackplaneApi "github.com/openshift/backplane-api/pkg/client"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/utils"
)

var GetBackplaneClusterFromConfig = utils.DefaultClusterUtils.GetBackplaneClusterFromConfig
var GetBackplaneConfiguration = config.GetBackplaneConfiguration


var credentialArgs struct {
	backplaneURL string
	output       string
}

type CredentialsResponse interface {
	// String returns a friendly message outlining how users can setup cloud environment access
	String() string

	// fmtExport sets environment variables for users to export to setup cloud environment access
	fmtExport() string
}

type AWSCredentialsResponse struct {
	AccessKeyId     string `json:"AccessKeyId" yaml:"AccessKeyId"`
	SecretAccessKey string `json:"SecretAccessKey" yaml:"SecretAccessKey"`
	SessionToken    string `json:"SessionToken" yaml:"SessionToken"`
	Region          string `json:"Region" yaml:"Region"`
	Expiration      string `json:"Expiration" yaml:"Expiration"`
}

type GCPCredentialsResponse struct {
	ProjectId string `json:"project_id" yaml:"project_id"`
}

const (
	// format strings for printing AWS credentials as a string or as environment variables
	awsCredentialsStringFormat = `Temporary Credentials:
  AccessKeyId: %s
  SecretAccessKey: %s
  SessionToken: %s
  Region: %s
  Expires: %s`
	awsExportFormat = `export AWS_ACCESS_KEY_ID=%s
export AWS_SECRET_ACCESS_KEY=%s
export AWS_SESSION_TOKEN=%s
export AWS_DEFAULT_REGION=%s`

	// format strings for printing GCP credentials as a string or as environment variables
	gcpCredentialsStringFormat = `If this is your first time, run "gcloud auth login" and then
gcloud config set project %s`
	gcpExportFormat = `export CLOUDSDK_CORE_PROJECT=%s`
)

func (r *AWSCredentialsResponse) String() string {
	return fmt.Sprintf(awsCredentialsStringFormat, r.AccessKeyId, r.SecretAccessKey, r.SessionToken, r.Region, r.Expiration)
}

func (r *AWSCredentialsResponse) fmtExport() string {
	return fmt.Sprintf(awsExportFormat, r.AccessKeyId, r.SecretAccessKey, r.SessionToken, r.Region)
}

func (r *GCPCredentialsResponse) String() string {
	return fmt.Sprintf(gcpCredentialsStringFormat, r.ProjectId)
}

func (r *GCPCredentialsResponse) fmtExport() string {
	return fmt.Sprintf(gcpExportFormat, r.ProjectId)
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

	clusterId, clusterName, err := utils.DefaultOCMInterface.GetTargetCluster(clusterKey)
	if err != nil {
		return err
	}

	cluster, err := utils.DefaultOCMInterface.GetClusterInfoByID(clusterId)
	if err != nil {
		return fmt.Errorf("failed to get cluster info for %s: %w", clusterId, err)
	}

	cloudProvider := utils.DefaultClusterUtils.GetCloudProvider(cluster)

	logger.WithFields(logger.Fields{
		"ID":   clusterId,
		"Name": clusterName}).Infoln("Target cluster")

	// ============Get Backplane URl ==========================
	bpURL := ""
	if credentialArgs.backplaneURL != "" {
		bpURL = credentialArgs.backplaneURL
	} else {
		// Get Backplane configuration
		bpConfig, err := GetBackplaneConfiguration()
		if err != nil || bpConfig.URL == "" {
			return fmt.Errorf("can't find backplane url: %w", err)
		}
		bpURL = bpConfig.URL
		logger.Infof("Using backplane URL: %s\n", bpURL)
	}

	// ======== Call Endpoint ==================================
	logger.Debugln("Getting Cloud Credentials")

	credsResp, _ := getCloudCredential(bpURL, clusterId)

	// ======== Render cloud credentials =======================
	switch cloudProvider {
	case "aws":
		cliResp := &AWSCredentialsResponse{}
		if err := json.Unmarshal([]byte(*credsResp.JSON200.Credentials), cliResp); err != nil {
			return fmt.Errorf("unable to unmarshal AWS credentials response from backplane %s: %w", *credsResp.JSON200.Credentials, err)
		}
		cliResp.Region = *credsResp.JSON200.Region
		creds, err := renderCloudCredentials(credentialArgs.output, cliResp)
		if err != nil {
			return err
		}
		fmt.Println(creds)
		return nil
	case "gcp":
		cliResp := &GCPCredentialsResponse{}
		if err := json.Unmarshal([]byte(*credsResp.JSON200.Credentials), cliResp); err != nil {
			return fmt.Errorf("unable to unmarshal GCP credentials response from backplane %s: %w", *credsResp.JSON200.Credentials, err)
		}
		creds, err := renderCloudCredentials(credentialArgs.output, cliResp)
		if err != nil {
			return err
		}
		fmt.Println(creds)
		return nil
	default:
		return fmt.Errorf("unsupported cloud provider: %s", cloudProvider)
	}
}

// getCloudCredential returns Cloud Credentials Response
func getCloudCredential(backplaneURL string, clusterId string) (*BackplaneApi.GetCloudCredentialsResponse, error) {
	client, err := utils.DefaultClientUtils.GetBackplaneClient(backplaneURL)
	if err != nil {
		return nil, err
	}


	resp, err := client.GetCloudCredentials(context.TODO(), clusterId)

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


// renderCloudCredentials displays the results of `ocm backplane cloud credentials` for AWS clusters
func renderCloudCredentials(outputFormat string, creds CredentialsResponse) (string, error) {
	switch outputFormat {
	case "env":
		return creds.fmtExport(), nil
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
