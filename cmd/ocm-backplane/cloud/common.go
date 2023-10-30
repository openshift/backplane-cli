package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	v1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift/backplane-cli/pkg/awsutil"
	"github.com/openshift/backplane-cli/pkg/utils"
	"io"
	"net/http"
)

const OldFlowSupportRole = "role/RH-Technical-Support-Access"

var StsClientWithProxy = awsutil.StsClientWithProxy
var AssumeRoleWithJWT = awsutil.AssumeRoleWithJWT
var NewStaticCredentialsProvider = credentials.NewStaticCredentialsProvider
var AssumeRoleSequence = awsutil.AssumeRoleSequence

type assumeChainResponse struct {
	AssumptionSequence []namedRoleArn `json:"assumptionSequence"`
}

type namedRoleArn struct {
	Name string `json:"name"`
	Arn  string `json:"arn"`
}

func getIsolatedCredentials(clusterID string) (aws.Credentials, error) {
	if clusterID == "" {
		return aws.Credentials{}, errors.New("must provide non-empty cluster ID")
	}

	ocmToken, err := utils.DefaultOCMInterface.GetOCMAccessToken()
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to retrieve OCM token: %w", err)
	}

	email, err := utils.GetStringFieldFromJWT(*ocmToken, "email")
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("unable to extract email from given token: %w", err)
	}

	bpConfig, err := GetBackplaneConfiguration()
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("error retrieving backplane configuration: %w", err)
	}

	if bpConfig.AssumeInitialArn == "" {
		return aws.Credentials{}, errors.New("backplane config is missing required `assume-initial-arn` property")
	}

	initialClient, err := StsClientWithProxy(bpConfig.ProxyURL)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to create sts client: %w", err)
	}

	seedCredentials, err := AssumeRoleWithJWT(*ocmToken, bpConfig.AssumeInitialArn, initialClient)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to assume role using JWT: %w", err)
	}

	backplaneClient, err := utils.DefaultClientUtils.MakeRawBackplaneAPIClientWithAccessToken(bpConfig.URL, *ocmToken)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to create backplane client with access token: %w", err)
	}

	response, err := backplaneClient.GetAssumeRoleSequence(context.TODO(), clusterID)
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

	targetCredentials, err := AssumeRoleSequence(email, seedClient, roleAssumeSequence, bpConfig.ProxyURL, awsutil.DefaultSTSClientProviderFunc)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to assume role sequence: %w", err)
	}
	return targetCredentials, nil
}

func isIsolatedBackplaneAccess(cluster *v1.Cluster) (bool, error) {
	if clusterAws, ok := cluster.GetAWS(); ok {
		if clusterAwsSts, ok := clusterAws.GetSTS(); ok {
			if clusterAwsSts.Enabled() {
				stsSupportJumpRole, err := utils.DefaultOCMInterface.GetStsSupportJumpRoleARN(cluster.ID())
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
		}
	}
	return false, nil
}
