package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	bpCredentials "github.com/openshift/backplane-cli/pkg/credentials"
	"github.com/openshift/backplane-cli/pkg/ocm"
	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	createClientSet = func(c *rest.Config) (kubernetes.Interface, error) { return kubernetes.NewForConfig(c) }
)

var ssmArgs struct {
	node string
}

var SSMSessionCmd = &cobra.Command{
	Use:   "ssm",
	Short: "Start an AWS SSM session for a node",
	Long:  "Start an AWS SSM session for the specified node provided to debug.",
	Args:  cobra.ExactArgs(0),
	RunE:  startSSMsession,
}

func init() {
	SSMSessionCmd.Flags().StringVar(&ssmArgs.node, "node", "", "Specify the node name to start the SSM session.")
}

func fetchCloudCredentials() (*bpCredentials.AWSCredentialsResponse, error) {
	var clusterKey string
	clusterInfo, err := GetBackplaneClusterFromConfig()
	if err != nil {
		return nil, fmt.Errorf("expected exactly one cluster: %w", err)
	}
	clusterKey = clusterInfo.ClusterID

	clusterID, clusterName, err := ocm.DefaultOCMInterface.GetTargetCluster(clusterKey)
	if err != nil {
		return nil, fmt.Errorf("expected exactly one cluster: %w", err)
	}

	cluster, err := ocm.DefaultOCMInterface.GetClusterInfoByID(clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster info for %s: %w", clusterID, err)
	}

	logger.WithFields(logger.Fields{
		"ID":   clusterID,
		"Name": clusterName}).Infoln("Target cluster")

	backplaneConfig, err := config.GetBackplaneConfiguration()
	if err != nil {
		return nil, fmt.Errorf("failed to get backplane configuration: %w", err)
	}

	ocmConnection, err := ocm.DefaultOCMInterface.SetupOCMConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to create OCM connection: %w", err)
	}
	defer ocmConnection.Close()

	queryConfig := &QueryConfig{OcmConnection: ocmConnection, BackplaneConfiguration: backplaneConfig, Cluster: cluster}

	creds, err := queryConfig.GetCloudCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch cloud credentials: %w", err)
	}

	awsCreds, ok := creds.(*bpCredentials.AWSCredentialsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected credentials type: %T", creds)
	}

	logger.Info("Successfully fetched cloud credentials.")
	return awsCreds, nil
}

func getInstanceID(node string, config *rest.Config) (string, error) {
	logger.Infof("Fetching instance ID for node: %s", node)

	clientset, err := createClientSet(config)
	if err != nil {
		return "", fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	nodeDetails, err := clientset.CoreV1().Nodes().Get(context.TODO(), node, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get node %s: %w", node, err)
	}

	providerID := nodeDetails.Spec.ProviderID
	if providerID == "" {
		return "", fmt.Errorf("providerID is not set for node %s", node)
	}

	parts := strings.Split(providerID, "/")
	instanceID := parts[len(parts)-1]

	return instanceID, nil
}

func startSSMsession(cmd *cobra.Command, argv []string) error {
	if ssmArgs.node == "" {
		return fmt.Errorf("--node flag is required")
	}

	// Fetch cloud credentials and export them as environment variables
	creds, err := fetchCloudCredentials()
	if err != nil {
		return fmt.Errorf("failed to fetch cloud credentials: %w", err)
	}

	// Set AWS credentials in environment variables
	os.Setenv("AWS_ACCESS_KEY_ID", creds.AccessKeyID)
	os.Setenv("AWS_SECRET_ACCESS_KEY", creds.SecretAccessKey)
	os.Setenv("AWS_SESSION_TOKEN", creds.SessionToken)

	kubeconfig, err := getCurrentKubeconfig()
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	instanceID, err := getInstanceID(ssmArgs.node, kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to get instance ID for node %s: %w", ssmArgs.node, err)
	}

	cfg, err := awsConfig.LoadDefaultConfig(context.TODO(), awsConfig.WithRegion("us-west-2"))
	if err != nil {
		return fmt.Errorf("unable to load SDK config: %v", err)
	}
	ssmClient := ssm.NewFromConfig(cfg)

	logger.Infof("Starting SSM session for node: %s with Instance ID: %s", ssmArgs.node, instanceID)

	input := &ssm.StartSessionInput{
		Target: aws.String(instanceID),
	}

	result, err := ssmClient.StartSession(context.TODO(), input)
	if err != nil {
		return fmt.Errorf("failed to start SSM session via SDK: %w", err)
	}

	// Ensure the session details are not nil
	if result.SessionId == nil || result.StreamUrl == nil || result.TokenValue == nil {
		return fmt.Errorf("session details are incomplete: SessionId=%v, StreamUrl=%v, TokenValue=%v", result.SessionId, result.StreamUrl, result.TokenValue)
	}

	// Log session details for debugging
	logger.Infof("SessionId: %v", *result.SessionId)
	logger.Infof("StreamUrl: %v", *result.StreamUrl)
	logger.Infof("TokenValue: %v", *result.TokenValue)

	// Prepare arguments for Session Manager Plugin
	sessionJSON, err := json.Marshal(map[string]string{
		"SessionId":  *result.SessionId,
		"StreamUrl":  *result.StreamUrl,
		"TokenValue": *result.TokenValue,
	})
	if err != nil {
		return fmt.Errorf("failed to serialize session details: %w", err)
	}

	validate_session_cmd := exec.Command("session-manager-plugin", "--version")
	err = validate_session_cmd.Run()
	if err != nil {
		return fmt.Errorf("session-manager-plugin is not installed. Please refer AWS doc: https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html")
	}

	cmdArgs := []string{"session-manager-plugin", string(sessionJSON), creds.Region, "StartSession"}
	pluginCmd := exec.Command(cmdArgs[0], cmdArgs[1:]...) //#nosec G204: Command arguments are trusted
	pluginCmd.Stdout = os.Stdout
	pluginCmd.Stderr = os.Stderr
	pluginCmd.Stdin = os.Stdin

	return pluginCmd.Run()
}

func getCurrentKubeconfig() (*rest.Config, error) {
	cf := genericclioptions.NewConfigFlags(true)
	config, err := cf.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("--node flag is required")
	}
	return config, nil
}
