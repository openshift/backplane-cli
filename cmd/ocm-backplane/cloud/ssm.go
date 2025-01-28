package cloud

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

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

	creds, err := fetchCloudCredentials()
	if err != nil {
		return fmt.Errorf("failed to fetch cloud credentials: %w", err)
	}

	kubeconfig, err := getCurrentKubeconfig()
	if err != nil {
		return err
	}

	os.Setenv("AWS_ACCESS_KEY_ID", creds.AccessKeyID)
	os.Setenv("AWS_SECRET_ACCESS_KEY", creds.SecretAccessKey)
	os.Setenv("AWS_SESSION_TOKEN", creds.SessionToken)

	instanceID, err := getInstanceID(ssmArgs.node, kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to get instance ID for node %s: %w", ssmArgs.node, err)
	}

	region := creds.Region

	logger.Infof("Starting SSM session for node: %s with Instance ID: %s", ssmArgs.node, instanceID)

	cmdArgs := []string{"ssm", "start-session", "--target", instanceID, "--region", region}

	awsCmd := exec.Command("aws", cmdArgs...)
	awsCmd.Stdout = os.Stdout
	awsCmd.Stderr = os.Stderr
	awsCmd.Stdin = os.Stdin

	if err := awsCmd.Run(); err != nil {
		return fmt.Errorf("failed to start AWS SSM session: %v", err)
	}

	return nil
}

func getCurrentKubeconfig() (*rest.Config, error) {
	cf := genericclioptions.NewConfigFlags(true)
	config, err := cf.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("--node flag is required")
	}
	return config, nil
}
