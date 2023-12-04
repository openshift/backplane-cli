package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ocmsdk "github.com/openshift-online/ocm-sdk-go"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	BackplaneApi "github.com/openshift/backplane-api/pkg/client"
	"github.com/openshift/backplane-cli/pkg/awsutil"
	"github.com/openshift/backplane-cli/pkg/backplaneapi"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	bpCredentials "github.com/openshift/backplane-cli/pkg/credentials"
	"github.com/openshift/backplane-cli/pkg/ocm"
	"github.com/openshift/backplane-cli/pkg/utils"
	logger "github.com/sirupsen/logrus"
)

const OldFlowSupportRole = "role/RH-Technical-Support-Access"

var StsClient = awsutil.StsClient
var AssumeRoleWithJWT = awsutil.AssumeRoleWithJWT
var NewStaticCredentialsProvider = credentials.NewStaticCredentialsProvider
var AssumeRoleSequence = awsutil.AssumeRoleSequence

// Wrapper for the configuration needed for cloud requests
type QueryConfig struct {
	config.BackplaneConfiguration
	OcmConnection *ocmsdk.Connection
	Cluster       *cmv1.Cluster
}

// GetAWSV2Config allows consumers to get an aws-sdk-go-v2 Config to programmatically access the AWS API
func (cfg *QueryConfig) GetAWSV2Config() (aws.Config, error) {
	if cfg.Cluster.CloudProvider().ID() != "aws" {
		return aws.Config{}, fmt.Errorf("only supported for the aws cloud provider, this cluster has: %s", cfg.Cluster.CloudProvider().ID())
	}
	creds, err := cfg.GetCloudCredentials()
	if err != nil {
		return aws.Config{}, err
	}

	awsCreds, ok := creds.(*bpCredentials.AWSCredentialsResponse)
	if !ok {
		return aws.Config{}, errors.New("unexpected error: failed to convert backplane creds to AWSCredentialsResponse")
	}

	return awsCreds.AWSV2Config()
}

// GetCloudCredentials returns Cloud Credentials Response
func (cfg *QueryConfig) GetCloudConsole() (*ConsoleResponse, error) {
	ocmToken, _, err := cfg.OcmConnection.Tokens()
	if err != nil {
		return nil, fmt.Errorf("unable to get token for ocm connection")
	}

	isolatedBackplane, err := isIsolatedBackplaneAccess(cfg.Cluster, cfg.OcmConnection)
	if err != nil {
		logger.Infof("failed to determine if the cluster is using isolated backplane access: %v", err)
		logger.Infof("for more information, try ocm get /api/clusters_mgmt/v1/clusters/%s/sts_support_jump_role", cfg.Cluster.ID())
		logger.Infof("attempting to fallback to %s", OldFlowSupportRole)
	}

	if isolatedBackplane {
		logger.Debugf("cluster is using isolated backplane")
		targetCredentials, err := cfg.getIsolatedCredentials(ocmToken)
		if err != nil {
			// TODO: This fallback should be removed in the future
			// TODO: when we are more confident in our ability to access clusters using the isolated flow
			logger.Infof("failed to assume role with isolated backplane flow: %v", err)
			logger.Infof("attempting to fallback to %s", OldFlowSupportRole)
			return cfg.getCloudConsoleFromPublicAPI(ocmToken)
		}

		resp, err := awsutil.GetSigninToken(targetCredentials, cfg.Cluster.Region().ID())
		if err != nil {
			return nil, fmt.Errorf("failed to get signin token: %w", err)
		}

		signinFederationURL, err := awsutil.GetConsoleURL(resp.SigninToken, cfg.Cluster.Region().ID())
		if err != nil {
			return nil, fmt.Errorf("failed to generate console url: %w", err)
		}
		return &ConsoleResponse{ConsoleLink: signinFederationURL.String()}, nil
	}

	return cfg.getCloudConsoleFromPublicAPI(ocmToken)
}

// GetCloudConsole returns console response calling to public Backplane API
func (cfg *QueryConfig) getCloudConsoleFromPublicAPI(ocmToken string) (*ConsoleResponse, error) {
	logger.Debugln("Getting Cloud Console")

	client, err := backplaneapi.DefaultClientUtils.GetBackplaneClient(cfg.BackplaneConfiguration.URL, ocmToken, cfg.BackplaneConfiguration.ProxyURL)
	if err != nil {
		return nil, err
	}
	resp, err := client.GetCloudConsole(context.TODO(), cfg.Cluster.ID())
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

// GetCloudCredentials returns Cloud Credentials Response
func (cfg *QueryConfig) GetCloudCredentials() (bpCredentials.Response, error) {
	ocmToken, _, err := cfg.OcmConnection.Tokens()
	if err != nil {
		return nil, fmt.Errorf("unable to get token for ocm connection")
	}

	isolatedBackplane, err := isIsolatedBackplaneAccess(cfg.Cluster, cfg.OcmConnection)
	if err != nil {
		logger.Infof("failed to determine if the cluster is using isolated backplane access: %v", err)
		logger.Infof("for more information, try ocm get /api/clusters_mgmt/v1/clusters/%s/sts_support_jump_role", cfg.Cluster.ID())
		logger.Infof("attempting to fallback to %s", OldFlowSupportRole)
	}

	if isolatedBackplane {
		logger.Debugf("cluster is using isolated backplane")
		targetCredentials, err := cfg.getIsolatedCredentials(ocmToken)
		if err != nil {
			// TODO: This fallback should be removed in the future
			// TODO: when we are more confident in our ability to access clusters using the isolated flow
			logger.Infof("failed to assume role with isolated backplane flow: %v", err)
			logger.Infof("attempting to fallback to %s", OldFlowSupportRole)
			return cfg.getCloudCredentialsFromBackplaneAPI(ocmToken)
		}

		return &bpCredentials.AWSCredentialsResponse{
			AccessKeyID:     targetCredentials.AccessKeyID,
			SecretAccessKey: targetCredentials.SecretAccessKey,
			SessionToken:    targetCredentials.SessionToken,
			Expiration:      targetCredentials.Expires.String(),
			Region:          cfg.Cluster.Region().ID(),
		}, nil
	}

	return cfg.getCloudCredentialsFromBackplaneAPI(ocmToken)
}

func (cfg *QueryConfig) getCloudCredentialsFromBackplaneAPI(ocmToken string) (bpCredentials.Response, error) {
	client, err := backplaneapi.DefaultClientUtils.GetBackplaneClient(cfg.BackplaneConfiguration.URL, ocmToken, cfg.BackplaneConfiguration.ProxyURL)
	if err != nil {
		return nil, err
	}

	resp, err := client.GetCloudCredentials(context.TODO(), cfg.Cluster.ID())
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

	switch cfg.Cluster.CloudProvider().ID() {
	case "aws":
		cliResp := &bpCredentials.AWSCredentialsResponse{}
		if err := json.Unmarshal([]byte(*credsResp.JSON200.Credentials), cliResp); err != nil {
			return nil, fmt.Errorf("unable to unmarshal AWS credentials response from backplane %s: %w", *credsResp.JSON200.Credentials, err)
		}
		cliResp.Region = cfg.Cluster.Region().ID()
		return cliResp, nil
	case "gcp":
		cliResp := &bpCredentials.GCPCredentialsResponse{}
		if err := json.Unmarshal([]byte(*credsResp.JSON200.Credentials), cliResp); err != nil {
			return nil, fmt.Errorf("unable to unmarshal GCP credentials response from backplane %s: %w", *credsResp.JSON200.Credentials, err)
		}
		return cliResp, nil
	default:
		return nil, fmt.Errorf("unsupported cloud provider: %s", cfg.Cluster.CloudProvider().ID())
	}
}

type assumeChainResponse struct {
	AssumptionSequence []namedRoleArn `json:"assumptionSequence"`
}

type namedRoleArn struct {
	Name string `json:"name"`
	Arn  string `json:"arn"`
}

func (cfg *QueryConfig) getIsolatedCredentials(ocmToken string) (aws.Credentials, error) {
	if cfg.Cluster.ID() == "" {
		return aws.Credentials{}, errors.New("must provide non-empty cluster ID")
	}

	email, err := utils.GetStringFieldFromJWT(ocmToken, "email")
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("unable to extract email from given token: %w", err)
	}

	if cfg.BackplaneConfiguration.AssumeInitialArn == "" {
		return aws.Credentials{}, errors.New("backplane config is missing required `assume-initial-arn` property")
	}

	initialClient, err := StsClient(cfg.BackplaneConfiguration.ProxyURL)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to create sts client: %w", err)
	}

	seedCredentials, err := AssumeRoleWithJWT(ocmToken, cfg.BackplaneConfiguration.AssumeInitialArn, initialClient)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to assume role using JWT: %w", err)
	}

	backplaneClient, err := backplaneapi.DefaultClientUtils.GetBackplaneClient(cfg.BackplaneConfiguration.URL, ocmToken, cfg.BackplaneConfiguration.ProxyURL)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to create backplane client with access token: %w", err)
	}

	response, err := backplaneClient.GetAssumeRoleSequence(context.TODO(), cfg.Cluster.ID())
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to fetch arn sequence: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return aws.Credentials{}, fmt.Errorf("failed to fetch arn sequence: %v", response.Status)
	}

	bytes, err := io.ReadAll(response.Body)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to read backplane API response body: %w", err)
	}

	roleChainResponse := &assumeChainResponse{}
	err = json.Unmarshal(bytes, roleChainResponse)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	roleAssumeSequence := make([]string, 0, len(roleChainResponse.AssumptionSequence))
	for _, namedRoleArn := range roleChainResponse.AssumptionSequence {
		roleAssumeSequence = append(roleAssumeSequence, namedRoleArn.Arn)
	}

	seedClient := sts.NewFromConfig(aws.Config{
		Region:      "us-east-1",
		Credentials: NewStaticCredentialsProvider(seedCredentials.AccessKeyID, seedCredentials.SecretAccessKey, seedCredentials.SessionToken),
	})

	targetCredentials, err := AssumeRoleSequence(email, seedClient, roleAssumeSequence, cfg.BackplaneConfiguration.ProxyURL, awsutil.DefaultSTSClientProviderFunc)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to assume role sequence: %w", err)
	}
	return targetCredentials, nil
}

func isIsolatedBackplaneAccess(cluster *cmv1.Cluster, ocmConnection *ocmsdk.Connection) (bool, error) {
	if cluster.AWS().STS().Enabled() {
		stsSupportJumpRole, err := ocm.DefaultOCMInterface.GetStsSupportJumpRoleARN(ocmConnection, cluster.ID())
		if err != nil {
			return false, fmt.Errorf("failed to get sts support jump role ARN for cluster %v: %w", cluster.ID(), err)
		}
		supportRoleArn, err := arn.Parse(stsSupportJumpRole)
		if err != nil {
			return false, fmt.Errorf("failed to parse ARN for jump role %v: %w", stsSupportJumpRole, err)
		}
		if supportRoleArn.Resource != OldFlowSupportRole {
			return true, nil
		}
	}

	return false, nil
}
