package cloud

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/openshift/backplane-cli/pkg/cli/config"
	bpCredentials "github.com/openshift/backplane-cli/pkg/credentials"
	"github.com/openshift/backplane-cli/pkg/ocm"
	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var Node string

var SSMSessionCmd = &cobra.Command{
	Use:   "ssm",
	Short: "Start an AWS SSM session for a node",
	Long:  "Start an AWS SSM session for the specified node provided to debug.",
	Args:  cobra.ExactArgs(0),
	RunE:  startSSMsession,
}

func init() {
	SSMSessionCmd.Flags().StringVar(&Node, "node", "", "Specify the node name to start the SSM session.")
}

// fetchCloudCredentials fetches AWS credentials for the currently logged-in cluster.
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

	// Get backplane configuration
	backplaneConfig, err := config.GetBackplaneConfiguration()
	if err != nil {
		return nil, fmt.Errorf("failed to get backplane configuration: %w", err)
	}

	// Initialize OCM connection
	ocmConnection, err := ocm.DefaultOCMInterface.SetupOCMConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to create OCM connection: %w", err)
	}
	defer ocmConnection.Close()

	// Create query configuration
	queryConfig := &QueryConfig{OcmConnection: ocmConnection, BackplaneConfiguration: backplaneConfig, Cluster: cluster}

	// Fetch cloud credentials
	creds, err := queryConfig.GetCloudCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch cloud credentials: %w", err)
	}

	// Cast to AWS credentials response
	awsCreds, ok := creds.(*bpCredentials.AWSCredentialsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected credentials type: %T", creds)
	}

	logger.Info("Successfully fetched cloud credentials.")
	return awsCreds, nil
}

// getInstanceID fetches the instance id of the given node
func getInstanceID(Node string) (string, error) {
	logger.Infof("Fetching instance ID for node: %s", Node)
	command := fmt.Sprintf("oc get node %s -o jsonpath='{.spec.providerID}' | awk -F/ '{print $NF}'", Node)

	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.Output()
	if err != nil {
		logger.Errorf("Failed to fetch instance ID: %v", err)
		return "", err
	}

	instanceID := strings.TrimSpace(string(output))
	if instanceID == "" {
		return "", fmt.Errorf("no Instance ID retrieved for node %s", Node)
	}

	return instanceID, nil
}

// startSSMsession function startes a SSM session for a given node in HCP cluster
func startSSMsession(cmd *cobra.Command, argv []string) error {
	// Check if Node name is present
	if Node == "" {
		return fmt.Errorf("--node flag is required")
	}

	// Fetch AWS credentials
	creds, err := fetchCloudCredentials()
	if err != nil {
		return fmt.Errorf("failed to fetch cloud credentials: %w", err)
	}

	// Set AWS environment variables
	os.Setenv("AWS_ACCESS_KEY_ID", creds.AccessKeyID)
	os.Setenv("AWS_SECRET_ACCESS_KEY", creds.SecretAccessKey)
	os.Setenv("AWS_SESSION_TOKEN", creds.SessionToken)

	// Fetch instance ID
	instanceID, err := getInstanceID(Node)
	if err != nil {
		return fmt.Errorf("failed to get instance ID for node %s: %w", Node, err)
	}

	logger.Infof("Starting SSM session for node: %s with Instance ID: %s", Node, instanceID)

	if _, err := exec.LookPath("aws"); err != nil {
		return fmt.Errorf("AWS CLI is not installed or not found in PATH. Please install it and try again")
	}

	// Construct the AWS SSM command
	command := fmt.Sprintf("aws ssm start-session --target %s", instanceID)
	logger.Infof("Executing command: %s", command)

	cmdExec := exec.Command("sh", "-c", command)
	cmdExec.Stdout = os.Stdout
	cmdExec.Stderr = os.Stderr

	if err := cmdExec.Run(); err != nil {
		logger.Errorf("Failed to start SSM session: %v", err)
		fmt.Printf("To manually start the session, run: %s\n", command)
		return err
	}

	return nil
}
