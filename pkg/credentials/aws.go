package credentials

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	bpconfig "github.com/openshift/backplane-cli/pkg/cli/config"
)

const (
	// AwsCredentialsStringFormat format strings for printing AWS credentials as a string or as environment variables
	AwsCredentialsStringFormat = `Temporary Credentials:
  AccessKeyID: %s
  SecretAccessKey: %s
  SessionToken: %s
  Region: %s
  Expires: %s`
	AwsExportFormat = `export AWS_ACCESS_KEY_ID=%s
export AWS_SECRET_ACCESS_KEY=%s
export AWS_SESSION_TOKEN=%s
export AWS_DEFAULT_REGION=%s`
)

type AWSCredentialsResponse struct {
	AccessKeyID     string `json:"AccessKeyID" yaml:"AccessKeyID"`
	SecretAccessKey string `json:"SecretAccessKey" yaml:"SecretAccessKey"`
	SessionToken    string `json:"SessionToken" yaml:"SessionToken"`
	Region          string `json:"Region" yaml:"Region"`
	Expiration      string `json:"Expiration" yaml:"Expiration"`
}

func (r *AWSCredentialsResponse) String() string {
	return fmt.Sprintf(AwsCredentialsStringFormat, r.AccessKeyID, r.SecretAccessKey, r.SessionToken, r.Region, r.Expiration)
}

func (r *AWSCredentialsResponse) FmtExport() string {
	return fmt.Sprintf(AwsExportFormat, r.AccessKeyID, r.SecretAccessKey, r.SessionToken, r.Region)
}

// AWSV2Config returns an aws-sdk-go-v2 config that can be used to programmatically access the AWS API
func (r *AWSCredentialsResponse) AWSV2Config() (aws.Config, error) {
	bpConfig, err := bpconfig.GetBackplaneConfiguration()
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load backplane config file: %w", err)
	}

	if bpConfig.ProxyURL != nil {
		proxyURL, err := url.Parse(*bpConfig.ProxyURL)
		if err != nil {
			return aws.Config{}, fmt.Errorf("failed to parse proxy_url from backplane config file: %w", err)
		}

		httpClient := awshttp.NewBuildableClient().WithTransportOptions(func(tr *http.Transport) {
			tr.Proxy = http.ProxyURL(proxyURL)
		})

		return config.LoadDefaultConfig(context.Background(),
			config.WithHTTPClient(httpClient),
			config.WithRegion(r.Region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(r.AccessKeyID, r.SecretAccessKey, r.SessionToken)),
		)
	}

	return config.LoadDefaultConfig(context.Background(),
		config.WithRegion(r.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(r.AccessKeyID, r.SecretAccessKey, r.SessionToken)),
	)
}
