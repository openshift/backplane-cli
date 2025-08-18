package cloud

import (
	"context"
	"net"
	"net/url"
	"slices"
	"strings"

	//nolint:gosec
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
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

const (
	OldFlowSupportRole  = "role/RH-Technical-Support-Access"
	CustomerRoleArnName = "Target-Role-Arn"
	OrgRoleArnName      = "Org-Role-Arn"
)

var StsClient = awsutil.StsClient
var AssumeRoleWithJWT = awsutil.AssumeRoleWithJWT
var NewStaticCredentialsProvider = credentials.NewStaticCredentialsProvider
var AssumeRoleSequence = awsutil.AssumeRoleSequence
var GetCallerIdentity = func(client *sts.Client) error {
	_, err := client.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
	return err
}

// CheckEgressIP checks the egress IP of the client
// This is a wrapper around checkEgressIPImpl to allow for easy mocking
var CheckEgressIP = checkEgressIPImpl

// QueryConfig Wrapper for the configuration needed for cloud requests
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

// GetCloudConsole returns Cloud Credentials Response
func (cfg *QueryConfig) GetCloudConsole() (*ConsoleResponse, error) {
	ocmToken, _, err := cfg.OcmConnection.Tokens()
	if err != nil {
		return nil, fmt.Errorf("unable to get token for ocm connection")
	}

	isolatedBackplane, err := isIsolatedBackplaneAccess(cfg.Cluster, cfg.OcmConnection)
	if err != nil {
		return nil, fmt.Errorf("failed to determine if cluster is using isolated backlpane access: %w", err)
	}

	if isolatedBackplane {
		logger.Debugf("cluster is using isolated backplane")
		targetCredentials, err := cfg.getIsolatedCredentials(ocmToken)
		if err != nil {
			return nil, fmt.Errorf("failed to assume role with isolated backplane flow: %w", err)
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
	} else {
		return cfg.getCloudConsoleFromPublicAPI(ocmToken)
	}
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
		return nil, fmt.Errorf("failed to determine if cluster is using isolated backlpane access: %w", err)
	}

	if isolatedBackplane {
		logger.Debugf("cluster is using isolated backplane")
		targetCredentials, err := cfg.getIsolatedCredentials(ocmToken)
		if err != nil {
			return nil, fmt.Errorf("failed to assume role with isolated backplane flow: %w", err)
		}

		return &bpCredentials.AWSCredentialsResponse{
			AccessKeyID:     targetCredentials.AccessKeyID,
			SecretAccessKey: targetCredentials.SecretAccessKey,
			SessionToken:    targetCredentials.SessionToken,
			Expiration:      targetCredentials.Expires.String(),
			Region:          cfg.Cluster.Region().ID(),
		}, nil
	} else {
		return cfg.getCloudCredentialsFromBackplaneAPI(ocmToken)
	}
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
	AssumptionSequence      []namedRoleArn `json:"assumptionSequence"`
	CustomerRoleSessionName string         `json:"customerRoleSessionName"`
	SessionPolicyArn        string         `json:"sessionPolicyArn"` // SessionPolicyArn is the ARN of the session policy
}

type namedRoleArn struct {
	Name string `json:"name"`
	Arn  string `json:"arn"`
}

func (cfg *QueryConfig) getIsolatedCredentials(ocmToken string) (aws.Credentials, error) {
	const (
		productionOCMUrl            = "https://api.openshift.com"
		productionAssumeInitialArn  = "arn:aws:iam::922711891673:role/SRE-Support-Role"
		stagingOCMUrl               = "https://api.stage.openshift.com"
		stagingAssumeInitialArn     = "arn:aws:iam::277304166082:role/SRE-Support-Role"
		integrationOCMUrl           = "https://api.integration.openshift.com"
		integrationAssumeInitialArn = "arn:aws:iam::277304166082:role/SRE-Support-Role"
	)

	if cfg.Cluster.ID() == "" {
		return aws.Credentials{}, errors.New("must provide non-empty cluster ID")
	}

	email, err := utils.GetStringFieldFromJWT(ocmToken, "email")
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("unable to extract email from given token: %w", err)
	}

	if cfg.BackplaneConfiguration.AssumeInitialArn == "" {
		// If not provided as an override, attempt to automatically set this based on OCM url
		switch cfg.OcmConnection.URL() {
		case productionOCMUrl:
			cfg.BackplaneConfiguration.AssumeInitialArn = productionAssumeInitialArn
		case stagingOCMUrl:
			cfg.BackplaneConfiguration.AssumeInitialArn = stagingAssumeInitialArn
		case integrationOCMUrl:
			cfg.BackplaneConfiguration.AssumeInitialArn = integrationAssumeInitialArn
		default:
			logger.Infof("failed to automatically set assume-initial-arn based on OCM url: %s", cfg.OcmConnection.URL())
			return aws.Credentials{}, errors.New("backplane config is missing required `assume-initial-arn` property")
		}

	} else if !slices.Contains(
		[]string{
			productionAssumeInitialArn,
			stagingAssumeInitialArn,
			integrationAssumeInitialArn,
		},
		cfg.BackplaneConfiguration.AssumeInitialArn,
	) {
		logger.Warnf("assume-initial-arn in backplane config is not set to a valid payer ARN, using: %s", cfg.BackplaneConfiguration.AssumeInitialArn)
		return aws.Credentials{}, fmt.Errorf("invalid assume-initial-arn: %s, must be one of: prod: %s, stage: %s, int: %s",
			cfg.BackplaneConfiguration.AssumeInitialArn,
			productionAssumeInitialArn,
			stagingAssumeInitialArn,
			integrationAssumeInitialArn,
		)

	}
	// Use AWS-specific proxy for initial STS client, fallback to regular proxy
	// Priority: 1) AWS proxy from config; 2) regular proxy from local backplane config
	stsProxyURL := cfg.BackplaneConfiguration.GetAwsProxy()
	initialClient, err := StsClient(stsProxyURL)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to create sts client: %w", err)
	}

	seedCredentials, err := AssumeRoleWithJWT(ocmToken, cfg.BackplaneConfiguration.AssumeInitialArn, initialClient)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to assume role using JWT: %w", err)
	}
	// Verify Sts connection with the seed credentials
	// Use AWS-specific proxy for STS operations
	awsProxyURL := cfg.BackplaneConfiguration.GetAwsProxy()

	var stsClientConfig aws.Config
	if awsProxyURL != nil {
		stsClientConfig = aws.Config{
			Region:      "us-east-1",
			Credentials: NewStaticCredentialsProvider(seedCredentials.AccessKeyID, seedCredentials.SecretAccessKey, seedCredentials.SessionToken),
			HTTPClient: &http.Client{
				Transport: &http.Transport{
					Proxy: func(*http.Request) (*url.URL, error) {
						return url.Parse(*awsProxyURL)
					},
				},
			},
		}
		logger.Debugf("Using AWS proxy for GetCallerIdentity: %s", *awsProxyURL)
	} else {
		stsClientConfig = aws.Config{
			Region:      "us-east-1",
			Credentials: NewStaticCredentialsProvider(seedCredentials.AccessKeyID, seedCredentials.SecretAccessKey, seedCredentials.SessionToken),
		}
	}

	stsClient := sts.NewFromConfig(stsClientConfig)
	err = GetCallerIdentity(stsClient)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("unable to connect to AWS STS endpoint (GetCallerIdentity failed): %w", err)
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
		return aws.Credentials{}, fmt.Errorf("failed to fetch arn sequence: %w", utils.TryPrintAPIError(response, false))
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

	inlinePolicy, err := verifyTrustedIPAndGetPolicy(cfg)
	if err != nil {
		return aws.Credentials{}, err
	}

	assumeRoleArnSessionSequence := make([]awsutil.RoleArnSession, 0, len(roleChainResponse.AssumptionSequence))
	for _, namedRoleArnEntry := range roleChainResponse.AssumptionSequence {
		roleArnSession := awsutil.RoleArnSession{RoleArn: namedRoleArnEntry.Arn}
		if namedRoleArnEntry.Name == CustomerRoleArnName || namedRoleArnEntry.Name == OrgRoleArnName {
			roleArnSession.RoleSessionName = roleChainResponse.CustomerRoleSessionName
		} else {
			roleArnSession.RoleSessionName = email
		}
		// Default to no policy ARNs
		roleArnSession.PolicyARNs = []types.PolicyDescriptorType{}
		if namedRoleArnEntry.Name == CustomerRoleArnName {
			roleArnSession.IsCustomerRole = true

			// Add the session policy ARN for selected roles
			if roleChainResponse.SessionPolicyArn != "" {
				logger.Debugf("Adding session policy ARN for role %s: %s", namedRoleArnEntry.Name, roleChainResponse.SessionPolicyArn)
				roleArnSession.PolicyARNs = []types.PolicyDescriptorType{
					{
						Arn: aws.String(roleChainResponse.SessionPolicyArn),
					},
				}
			} else {
				roleArnSession.Policy = &inlinePolicy
			}

		} else {
			roleArnSession.IsCustomerRole = false
		}
		roleArnSession.Name = namedRoleArnEntry.Name

		assumeRoleArnSessionSequence = append(assumeRoleArnSessionSequence, roleArnSession)
	}

	seedClient := sts.NewFromConfig(aws.Config{
		Region:      "us-east-1",
		Credentials: NewStaticCredentialsProvider(seedCredentials.AccessKeyID, seedCredentials.SecretAccessKey, seedCredentials.SessionToken),
	})

	// Use AWS-specific proxy for role sequence, fallback to regular proxy
	// Priority: 1) AWS proxy from config, 2) regular proxy from config
	roleSequenceProxyURL := cfg.BackplaneConfiguration.GetAwsProxy()
	targetCredentials, err := AssumeRoleSequence(
		seedClient,
		assumeRoleArnSessionSequence,
		roleSequenceProxyURL, // Use AWS proxy if configured, otherwise regular proxy
		awsutil.DefaultSTSClientProviderFunc,
	)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to assume role sequence: %w", err)
	}
	return targetCredentials, nil
}

// verifyTrustedIPAndGetPolicy verifies that the client IP is in the trusted IP range
// and returns the inline policy for the trusted IPs
func verifyTrustedIPAndGetPolicy(cfg *QueryConfig) (awsutil.PolicyDocument, error) {
	// Use AWS-specific proxy for egress IP check, fallback to regular proxy
	// Priority: 1) AWS proxy from config, 2) regular proxy from config
	var egressProxyURL *url.URL
	if proxyURLString := cfg.BackplaneConfiguration.GetAwsProxy(); proxyURLString != nil {
		var err error
		egressProxyURL, err = url.Parse(*proxyURLString)
		if err != nil {
			return awsutil.PolicyDocument{}, fmt.Errorf("failed to parse proxy for checkEgressIP: %w", err)
		}
		logger.Debugf("Using proxy for egress IP check: %s", *proxyURLString)
	}

	var httpClient *http.Client
	if egressProxyURL != nil {
		httpClient = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(egressProxyURL),
			},
		}
	} else {
		httpClient = &http.Client{}
	}
	clientIP, err := CheckEgressIP(httpClient, "https://checkip.amazonaws.com/")
	if err != nil {
		return awsutil.PolicyDocument{}, fmt.Errorf("failed to determine client IP: %w", err)
	}

	trustedRange, err := getTrustedIPList(cfg.OcmConnection)
	if err != nil {
		return awsutil.PolicyDocument{}, err
	}

	err = verifyIPTrusted(clientIP, trustedRange)
	if err != nil {
		return awsutil.PolicyDocument{}, err
	}

	inlinePolicy, err := getTrustedIPInlinePolicy(trustedRange)
	if err != nil {
		return awsutil.PolicyDocument{}, fmt.Errorf("failed to build inline policy: %w", err)
	}

	return inlinePolicy, nil
}

// checkEgressIPImpl checks the egress IP of the client
func checkEgressIPImpl(client *http.Client, url string) (net.IP, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch IP: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	ip := net.ParseIP(strings.TrimSpace(string(body)))
	if ip == nil {
		return nil, fmt.Errorf("failed to parse IP %s", body)
	}

	return ip, nil
}

func verifyIPTrusted(ip net.IP, trustedIPs awsutil.IPAddress) error {
	for _, trustedIP := range trustedIPs.SourceIp {
		parsedIP, _, err := net.ParseCIDR(trustedIP)
		if err != nil {
			return fmt.Errorf("failed to parse the given trusted IP: %w", err)
		}
		if parsedIP.Equal(ip) {
			return nil
		}
	}

	logger.Warnf("Your client side IP does not include in the given trusted IP range, " +
		"please consider using a different VPN instead")
	return fmt.Errorf("client IP %s is not in the trusted IP range", ip)
}

func getTrustedIPList(connection *ocmsdk.Connection) (awsutil.IPAddress, error) {
	IPList, err := ocm.DefaultOCMInterface.GetTrustedIPList(connection)
	if err != nil {
		return awsutil.IPAddress{}, fmt.Errorf("failed to fetch trusted IP list: %w", err)
	}

	sourceIPList := []string{}

	// (!) We are adding filtering on top of the trusted IP list here.
	// This additional filtering is to only add the IPs that are expected to access customer AWS accounts.
	// The reason behind this is a limitation in the trust policy length (2048 characters, see https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_iam-quotas.html).
	// This AWS restriction prevents us from adding all IPs, as the policy would become too long.
	for _, ip := range IPList.Items() {
		if ip.Enabled() {
			// TODO: Enable ourselves to pull the subset of IPs that need to be allowlisted in the trust policy.
			// Examples: add a field to the OCM trust policy to filter by subtype (vpn, proxy, automation), which could be re-used to filter here.
			// We don't want to hardcode these IPs in code, as these IPs are expected to change.

			// Proxy IPs
			if strings.HasPrefix(ip.ID(), "209.") ||
				strings.HasPrefix(ip.ID(), "66.") ||
				strings.HasPrefix(ip.ID(), "91.") {
				sourceIPList = append(sourceIPList, fmt.Sprintf("%s/32", ip.ID()))
			}

			// CAD stage IPs
			if strings.HasPrefix(ip.ID(), "3.216") ||
				strings.HasPrefix(ip.ID(), "34.227") ||
				strings.HasPrefix(ip.ID(), "98.85") {
				sourceIPList = append(sourceIPList, fmt.Sprintf("%s/32", ip.ID()))
			}

			// CAD Prod IPs
			if strings.HasPrefix(ip.ID(), "34.193") ||
				strings.HasPrefix(ip.ID(), "52.203") ||
				strings.HasPrefix(ip.ID(), "54.145") {
				sourceIPList = append(sourceIPList, fmt.Sprintf("%s/32", ip.ID()))
			}
		}

	}

	ipAddress := awsutil.IPAddress{
		SourceIp: sourceIPList,
	}

	return ipAddress, nil
}

func getTrustedIPInlinePolicy(IPAddress awsutil.IPAddress) (awsutil.PolicyDocument, error) {

	policy := awsutil.NewPolicyDocument(awsutil.PolicyVersion, []awsutil.PolicyStatement{})

	return policy.BuildPolicyWithRestrictedIP(IPAddress)
}

func isIsolatedBackplaneAccess(cluster *cmv1.Cluster, ocmConnection *ocmsdk.Connection) (bool, error) {
	// Check if the cluster's base domain ends with specified US GOV domains
	baseDomain := cluster.DNS().BaseDomain()
	if strings.HasSuffix(baseDomain, "devshiftusgov.com") || strings.HasSuffix(baseDomain, "openshiftusgov.com") {
		return false, nil
	}

	if cluster.Hypershift().Enabled() {
		return true, nil
	}

	if cluster.AWS().STS().Enabled() {
		stsSupportJumpRole, err := ocm.DefaultOCMInterface.GetStsSupportJumpRoleARN(ocmConnection, cluster.ID())
		if err != nil {
			return false, fmt.Errorf("failed to get sts support jump role ARN for cluster %v: %w", cluster.ID(), err)
		}
		supportRoleArn, err := arn.Parse(stsSupportJumpRole)
		if err != nil {
			return false, fmt.Errorf("failed to parse ARN for jump role %v: %w", stsSupportJumpRole, err)
		}
		return supportRoleArn.Resource != OldFlowSupportRole, nil
	} else {
		return false, nil
	}
}
