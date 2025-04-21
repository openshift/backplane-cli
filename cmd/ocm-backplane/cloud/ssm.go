package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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
	CreateClientSet       func(*rest.Config) (kubernetes.Interface, error)       = createClientSet
	GetInstanceID         func(node string, config *rest.Config) (string, error) = getInstanceID
	StartSSMsession       func(cmd *cobra.Command, argv []string) error          = startSSMsession
	ExecCommand           func(name string, arg ...string) *exec.Cmd             = exec.Command
	NewFromConfig         func(cfg aws.Config) SSMClient                         = NewFromConfigVar
	GetCurrentKubeconfig  func() (*rest.Config, error)                           = getCurrentKubeconfig
	FetchCloudCredentials func() (*bpCredentials.AWSCredentialsResponse, error)  = fetchCloudCredentials
)

func NewFromConfigVar(cfg aws.Config) SSMClient {
	return ssm.NewFromConfig(cfg)
}

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
	if err := SSMSessionCmd.MarkFlagRequired("node"); err != nil {
		fmt.Printf("Error marking flag as required: %v\n", err)
	}
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

func getInstanceID(nodeName string, config *rest.Config) (string, error) {
	clientset, err := CreateClientSet(config)
	if err != nil {
		return "", fmt.Errorf("failed to create client: %w", err)
	}

	node, err := clientset.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get node %s: %w", nodeName, err)
	}

	if node.Spec.ProviderID == "" {
		return "", fmt.Errorf("providerID is not set")
	}

	// Extract instance ID from ProviderID
	instanceID := strings.Split(node.Spec.ProviderID, "/")[len(strings.Split(node.Spec.ProviderID, "/"))-1]
	return instanceID, nil
}

func startSSMsession(cmd *cobra.Command, argv []string) error {
	// Check if session-manager-plugin is installed
	ValidateSessionCmd := ExecCommand("session-manager-plugin", "--version")
	err := ValidateSessionCmd.Run()
	if err != nil {
		return fmt.Errorf("failed to validate session-manager-plugin: %w. Please refer AWS doc to make sure session-manager-plugin is properly installed: https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html", err)
	}

	// Check if the node argument is provided
	if ssmArgs.node == "" {
		return fmt.Errorf("--node flag is required")
	}

	// Fetch proxy variable from backplane configuration
	backplaneConfig, err := GetBackplaneConfiguration()
	if err != nil {
		return fmt.Errorf("failed to get backplane configuration: %w", err)
	}
	proxyURL := backplaneConfig.ProxyURL
	if proxyURL == nil {
		return fmt.Errorf("proxy URL is not set in backplane configuration")
	}

	// Validate the proxy URL
	parsedProxyURL, err := url.Parse(*proxyURL)
	if err != nil {
		return fmt.Errorf("invalid proxy URL in backplane configuration: %w", err)
	}
	if parsedProxyURL.Scheme == "" || parsedProxyURL.Host == "" {
		return fmt.Errorf("invalid proxy URL in backplane configuration: missing scheme or host: %w", err)
	}

	// Log the proxy being used for debugging
	logger.Infof("Using proxy: %s", parsedProxyURL.String())

	// Create a custom HTTP client with the proxy configuration
	customHTTPClient := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(parsedProxyURL),
		},
	}

	// Fetch AWS credentials
	creds, err := FetchCloudCredentials()
	if err != nil {
		return fmt.Errorf("failed to fetch cloud credentials: %w", err)
	}

	// Set AWS credentials in environment variables
	os.Setenv("AWS_ACCESS_KEY_ID", creds.AccessKeyID)
	os.Setenv("AWS_SECRET_ACCESS_KEY", creds.SecretAccessKey)
	os.Setenv("AWS_SESSION_TOKEN", creds.SessionToken)

	// Load AWS SDK configuration with the custom HTTP client
	cfg, err := awsConfig.LoadDefaultConfig(
		context.TODO(),
		awsConfig.WithRegion(creds.Region),
		awsConfig.WithHTTPClient(customHTTPClient),
	)
	if err != nil {
		return fmt.Errorf("unable to load AWS SDK configuration: %w", err)
	}

	// Create SSM client
	ssmClient := NewFromConfig(cfg)

	// Get the current kubeconfig
	kubeconfig, err := getCurrentKubeconfig()
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	// Get the instance ID for the specified node
	instanceID, err := getInstanceID(ssmArgs.node, kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to get instance ID for node %s: %w", ssmArgs.node, err)
	}

	logger.Infof("Starting SSM session for node: %s with Instance ID: %s", ssmArgs.node, instanceID)

	// Start the SSM session
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

	// Start the ssm-session using the session-manager-plugin
	cmdArgs := []string{"session-manager-plugin", string(sessionJSON), creds.Region, "StartSession"}
	pluginCmd := ExecCommand(cmdArgs[0], cmdArgs[1:]...) //#nosec G204: Command arguments are trusted
	pluginCmd.Stdout = os.Stdout
	pluginCmd.Stderr = os.Stderr
	pluginCmd.Stdin = os.Stdin

	return pluginCmd.Run()
}

func getCurrentKubeconfig() (*rest.Config, error) {
	cf := genericclioptions.NewConfigFlags(true)
	config, err := cf.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	return config, nil
}

// Define SSMClient interface
type SSMClient interface {
	StartSession(ctx context.Context, params *ssm.StartSessionInput, optFns ...func(*ssm.Options)) (*ssm.StartSessionOutput, error)
}
