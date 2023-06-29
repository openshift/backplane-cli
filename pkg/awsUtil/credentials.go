package awsUtil

import (
	"encoding/json"
	"fmt"
	"sigs.k8s.io/yaml"
)

const awsExportFormat = `export AWS_ACCESS_KEY_ID=%s
export AWS_SECRET_ACCESS_KEY=%s
export AWS_SESSION_TOKEN=%s`

type AWSCredentialsResponse struct {
	AccessKeyId     string `json:"AccessKeyId" yaml:"AccessKeyId"`
	SecretAccessKey string `json:"SecretAccessKey" yaml:"SecretAccessKey"`
	SessionToken    string `json:"SessionToken" yaml:"SessionToken"`
	Region          string `json:"Region,omitempty" yaml:"Region,omitempty"`
	Expiration      string `json:"Expiration,omitempty" yaml:"Expiration,omitempty"`
}

func (r AWSCredentialsResponse) EnvFormat() string {
	return fmt.Sprintf(awsExportFormat, r.AccessKeyId, r.SecretAccessKey, r.SessionToken)
}

func (r AWSCredentialsResponse) RenderOutput(outputFormat string) (string, error) {
	switch outputFormat {
	case "env":
		return r.EnvFormat(), nil
	case "json":
		jsonBytes, err := json.Marshal(r)
		if err != nil {
			return "", fmt.Errorf("failed to render output as %v: %w", outputFormat, err)
		}
		return string(jsonBytes), nil
	case "yaml":
		yamlBytes, err := yaml.Marshal(r)
		if err != nil {
			return "", fmt.Errorf("failed to render output as %v: %w", outputFormat, err)
		}
		return string(yamlBytes), nil
	default:
		return "", fmt.Errorf("unsupported format %v", outputFormat)
	}
}
