package awsutil

import (
	"encoding/json"

	"github.com/aws/aws-sdk-go-v2/aws"
)

const (
	PolicyVersion = "2012-10-17"
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

type PolicyDocumentInterface interface {
	String() (string, error)
	BuildPolicyWithRestrictedIP(ipAddress IPAddress) (PolicyDocument, error)
}

type Condition struct {
	//nolint NotIpAddress is required from AWS Policy
	NotIpAddress IPAddress `json:"NotIpAddress"`
}

type IPAddress struct {
	//nolint SourceIp is required from AWS Policy
	SourceIp []string `json:"aws:SourceIp"`
}

func NewPolicyDocument(version string, statements []PolicyStatement) PolicyDocument {
	return PolicyDocument{
		Version:   version,
		Statement: statements,
	}
}

func (p PolicyDocument) String() string {
	policyBytes, _ := json.Marshal(p)

	return string(policyBytes)
}

func (p PolicyDocument) BuildPolicyWithRestrictedIP(ipAddress IPAddress) (PolicyDocument, error) {
	condition := Condition{
		NotIpAddress: ipAddress,
	}

	allAllow := NewPolicyStatement("AllowAll", "Allow", []string{"*"}).
		AddResource(aws.String("*")).
		AddCondition(nil)
	denyNonRHProxy := NewPolicyStatement("DenyNonRHProxy", "Deny", []string{"*"}).
		AddResource(aws.String("*")).
		AddCondition(&condition)
	p.Statement = []PolicyStatement{denyNonRHProxy, allAllow}
	return p, nil
}

func NewPolicyStatement(sid string, affect string, action []string) PolicyStatement {
	return PolicyStatement{
		Sid:    sid,
		Effect: affect,
		Action: action,
	}
}

func (ps PolicyStatement) AddResource(resource *string) PolicyStatement {
	ps.Resource = resource
	return ps
}

func (ps PolicyStatement) AddCondition(condition *Condition) PolicyStatement {
	ps.Condition = condition
	return ps
}
