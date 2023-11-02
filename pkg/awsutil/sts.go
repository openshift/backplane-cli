package awsutil

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/openshift/backplane-cli/pkg/utils"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	AwsFederatedSigninEndpointTemplate = "https://%v.signin.aws.amazon.com/federation"
	AwsConsoleURL                      = "https://console.aws.amazon.com/"
	DefaultIssuer                      = "Red Hat SRE"

	assumeRoleMaxRetries   = 3
	assumeRoleRetryBackoff = 5 * time.Second
)

type AWSFederatedSessionData struct {
	SessionID    string `json:"sessionId"`
	SessionKey   string `json:"sessionKey"`
	SessionToken string `json:"sessionToken"`
}

type AWSSigninTokenResponse struct {
	SigninToken string
}

var httpGetFunc = http.Get

func StsClientWithProxy(proxyURL string) (*sts.Client, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("us-east-1"), // We don't care about region here, but the API still wants to see one set
		config.WithHTTPClient(&http.Client{
			Transport: &http.Transport{
				Proxy: func(*http.Request) (*url.URL, error) {
					return url.Parse(proxyURL)
				},
			},
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load default AWS config: %w", err)
	}

	return sts.NewFromConfig(cfg), nil
}

// IdentityTokenValue is for retrieving an identity token from the given file name
type IdentityTokenValue string

// GetIdentityToken retrieves the JWT token from the file and returns the contents as a []byte
func (j IdentityTokenValue) GetIdentityToken() ([]byte, error) {
	return []byte(j), nil
}

func AssumeRoleWithJWT(jwt string, roleArn string, stsClient stscreds.AssumeRoleWithWebIdentityAPIClient) (aws.Credentials, error) {
	email, err := utils.GetStringFieldFromJWT(jwt, "email")
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("unable to extract email from given token: %w", err)
	}

	credentialsCache := aws.NewCredentialsCache(stscreds.NewWebIdentityRoleProvider(
		stsClient,
		roleArn,
		IdentityTokenValue(jwt),
		func(options *stscreds.WebIdentityRoleOptions) {
			options.RoleSessionName = email
		},
	))

	result, err := credentialsCache.Retrieve(context.TODO())
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("unable to assume the given role with the token provided: %w", err)
	}

	return result, nil
}

func AssumeRole(roleSessionName string, stsClient stscreds.AssumeRoleAPIClient, roleArn string) (aws.Credentials, error) {
	assumeRoleProvider := stscreds.NewAssumeRoleProvider(stsClient, roleArn, func(options *stscreds.AssumeRoleOptions) {
		options.RoleSessionName = roleSessionName
	})
	result, err := assumeRoleProvider.Retrieve(context.TODO())
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to assume role %v: %w", roleArn, err)
	}

	return result, nil
}

type STSClientProviderFunc func(optFns ...func(*config.LoadOptions) error) (stscreds.AssumeRoleAPIClient, error)

var DefaultSTSClientProviderFunc STSClientProviderFunc = func(optnFns ...func(options *config.LoadOptions) error) (stscreds.AssumeRoleAPIClient, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), optnFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to load default AWS config: %w", err)
	}
	return sts.NewFromConfig(cfg), nil
}

func AssumeRoleSequence(roleSessionName string, seedClient stscreds.AssumeRoleAPIClient, roleArnSequence []string, proxyURL string, stsClientProviderFunc STSClientProviderFunc) (aws.Credentials, error) {
	if len(roleArnSequence) == 0 {
		return aws.Credentials{}, errors.New("role ARN sequence cannot be empty")
	}

	nextClient := seedClient
	var lastCredentials aws.Credentials

	for i, roleArn := range roleArnSequence {
		result, err := AssumeRole(roleSessionName, nextClient, roleArn)
		retryCount := 0
		for err != nil {
			// IAM policy updates can take a few seconds to resolve, and the sts.Client in AWS' Go SDK doesn't refresh itself on retries.
			// https://github.com/aws/aws-sdk-go-v2/issues/2332
			if retryCount < assumeRoleMaxRetries {
				fmt.Println("Waiting for IAM policy changes to resolve...")
				time.Sleep(assumeRoleRetryBackoff)
				nextClient, err = createAssumeRoleSequenceClient(stsClientProviderFunc, lastCredentials, proxyURL)
				if err != nil {
					return aws.Credentials{}, fmt.Errorf("failed to create client with credentials for role %v: %w", roleArn, err)
				}
				result, err = AssumeRole(roleSessionName, nextClient, roleArn)
				retryCount++
			} else {
				return aws.Credentials{}, fmt.Errorf("failed to assume role %v: %w", roleArn, err)
			}
		}
		lastCredentials = result

		if i < len(roleArnSequence)-1 {
			nextClient, err = createAssumeRoleSequenceClient(stsClientProviderFunc, lastCredentials, proxyURL)
			if err != nil {
				return aws.Credentials{}, fmt.Errorf("failed to create client with credentials for role %v: %w", roleArn, err)
			}
		}
	}

	return lastCredentials, nil
}

func createAssumeRoleSequenceClient(stsClientProviderFunc STSClientProviderFunc, creds aws.Credentials, proxyURL string) (stscreds.AssumeRoleAPIClient, error) {
	return stsClientProviderFunc(
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken)),
		config.WithHTTPClient(&http.Client{
			Transport: &http.Transport{
				Proxy: func(*http.Request) (*url.URL, error) {
					return url.Parse(proxyURL)
				},
			},
		}),
		config.WithRegion("us-east-1"), // We don't care about region here, but the API still wants to see one set
	)
}

func GetSigninToken(awsCredentials aws.Credentials, region string) (*AWSSigninTokenResponse, error) {
	sessionData := AWSFederatedSessionData{
		SessionID:    awsCredentials.AccessKeyID,
		SessionKey:   awsCredentials.SecretAccessKey,
		SessionToken: awsCredentials.SessionToken,
	}

	data, err := json.Marshal(sessionData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal session data: %w", err)
	}

	federationParams := url.Values{}
	federationParams.Add("Action", "getSigninToken")
	federationParams.Add("SessionType", "json")
	federationParams.Add("Session", string(data))

	baseFederationURL, err := url.Parse(fmt.Sprintf(AwsFederatedSigninEndpointTemplate, region))
	if err != nil {
		return nil, fmt.Errorf("failed to parse aws federated signin endpoint: %w", err)
	}

	baseFederationURL.RawQuery = federationParams.Encode()

	res, err := httpGetFunc(baseFederationURL.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get signin token from %v: %w", baseFederationURL, err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get signin token from %v, status code %d", baseFederationURL, res.StatusCode)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var resp AWSSigninTokenResponse
	if err = json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal signin token response: %w", err)
	}

	return &resp, nil
}

func GetConsoleURL(signinToken string, region string) (*url.URL, error) {
	signinParams := url.Values{}
	signinParams.Add("Action", "login")
	signinParams.Add("Destination", AwsConsoleURL)
	signinParams.Add("Issuer", DefaultIssuer)
	signinParams.Add("SigninToken", signinToken)

	signInFederationURL, err := url.Parse(fmt.Sprintf(AwsFederatedSigninEndpointTemplate, region))
	if err != nil {
		return nil, fmt.Errorf("failed to parse federated signin endpoint: %w", err)
	}

	signInFederationURL.RawQuery = signinParams.Encode()
	return signInFederationURL, nil
}
