package awsutil

import (
	"encoding/json"

	"github.com/aws/aws-sdk-go-v2/aws"
)

type PolicyDocument struct {
	Version   string            `json:"Version"`
	Statement []PolicyStatement `json:"Statement"`
}

type PolicyStatement struct {
	Sid       string            `json:"Sid"`        // Statement ID
	Effect    string            `json:"Effect"`     // Allow or Deny
	Action    []string          `json:"Action"`     // allowed or denied action
	Principal map[string]string `json:",omitempty"` // principal that is allowed or denied
	Resource  *string           `json:",omitempty"` // object or objects that the statement covers
	Condition *Condition        `json:",omitempty"` // conditions for when a policy is in effect
}

type Condition struct {
	NotIpAddress IpAddress `json:"NotIpAddress"`
}

type IpAddress struct {
	SourceIp []string `json:"aws:SourceIp"`
}

func GetAssumeRoleInlinePolicy(ipAddress IpAddress) (string, error) {

	condition := Condition{
		NotIpAddress: ipAddress,
	}

	inlinePolicyDocument := PolicyDocument{
		Version: "2012-10-17",
		Statement: []PolicyStatement{
			{
				Sid:       "DenyOffVPN",
				Effect:    "Deny",
				Action:    []string{"*"},
				Resource:  aws.String("*"),
				Condition: &condition,
			},
			{
				Sid:       "AllowAll",
				Effect:    "Allow",
				Action:    []string{"*"},
				Resource:  aws.String("*"),
				Condition: nil,
			},
		},
	}
	policyBytes, err := json.Marshal(inlinePolicyDocument)
	if err != nil {
		return "", err
	}
	return string(policyBytes), nil
}
