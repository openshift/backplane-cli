package credentials

import "fmt"

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
