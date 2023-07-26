package awsutil

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"

	"github.com/openshift/backplane-cli/pkg/utils"
)

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

type STSRoleAssumer interface {
	AssumeRole(ctx context.Context, params *sts.AssumeRoleInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleOutput, error)
}

type STSRoleWithWebIdentityAssumer interface {
	AssumeRoleWithWebIdentity(ctx context.Context, params *sts.AssumeRoleWithWebIdentityInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleWithWebIdentityOutput, error)
}

func AssumeRoleWithJWT(jwt string, roleArn string, stsClient STSRoleWithWebIdentityAssumer) (*types.Credentials, error) {
	email, err := utils.GetStringFieldFromJWT(jwt, "email")
	if err != nil {
		return nil, fmt.Errorf("unable to extract email from given token: %w", err)
	}
	input := &sts.AssumeRoleWithWebIdentityInput{
		RoleArn:          aws.String(roleArn),
		RoleSessionName:  aws.String(email),
		WebIdentityToken: aws.String(jwt),
	}

	result, err := stsClient.AssumeRoleWithWebIdentity(context.TODO(), input)
	if err != nil {
		return nil, fmt.Errorf("unable to assume the given role with the token provided: %w", err)
	}

	return result.Credentials, nil
}

func AssumeRole(roleSessionName string, stsClient STSRoleAssumer, roleArn string) (*types.Credentials, error) {
	input := &sts.AssumeRoleInput{
		RoleArn:         aws.String(roleArn),
		RoleSessionName: aws.String(roleSessionName),
	}
	result, err := stsClient.AssumeRole(context.TODO(), input)
	if err != nil {
		return nil, fmt.Errorf("failed to assume role %v: %w", roleArn, err)
	}

	return result.Credentials, nil
}

type STSClientProviderFunc func(optFns ...func(*config.LoadOptions) error) (STSRoleAssumer, error)

var DefaultSTSClientProviderFunc STSClientProviderFunc = func(optnFns ...func(options *config.LoadOptions) error) (STSRoleAssumer, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), optnFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to load default AWS config: %w", err)
	}
	return sts.NewFromConfig(cfg), nil
}

func AssumeRoleSequence(roleSessionName string, seedClient STSRoleAssumer, roleArnSequence []string, proxyURL string, stsClientProviderFunc STSClientProviderFunc) (*types.Credentials, error) {
	if len(roleArnSequence) == 0 {
		return nil, errors.New("role ARN sequence cannot be empty")
	}

	nextClient := seedClient
	var lastCredentials *types.Credentials

	for i, roleArn := range roleArnSequence {
		result, err := AssumeRole(roleSessionName, nextClient, roleArn)
		if err != nil {
			return nil, fmt.Errorf("failed to assume role %v: %w", roleArn, err)
		}
		lastCredentials = result

		if i < len(roleArnSequence)-1 {
			nextClient, err = stsClientProviderFunc(
				config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(*lastCredentials.AccessKeyId, *lastCredentials.SecretAccessKey, *lastCredentials.SessionToken)),
				config.WithHTTPClient(&http.Client{
					Transport: &http.Transport{
						Proxy: func(*http.Request) (*url.URL, error) {
							return url.Parse(proxyURL)
						},
					},
				}),
				config.WithRetryer(func() aws.Retryer {
					return retry.NewStandard(func(options *retry.StandardOptions) {
						options.Retryables = append(options.Retryables, retry.RetryableHTTPStatusCode{
							Codes: map[int]struct{}{401: {}, 403: {}, 404: {}}, // Handle IAM eventual consistency because backplane api modifies trust policy
						})
						options.MaxAttempts = 5
						options.MaxBackoff = 20 * time.Second
					})
				}),
				config.WithRegion("us-east-1"), // We don't care about region here, but the API still wants to see one set
			)
			if err != nil {
				return nil, fmt.Errorf("failed to create client with credentials from role %v: %w", roleArn, err)
			}
		}
	}

	return lastCredentials, nil
}
