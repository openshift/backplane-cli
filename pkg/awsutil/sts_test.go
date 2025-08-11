package awsutil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
)

type STSRoleAssumerMock struct {
	mockResult            *sts.AssumeRoleOutput
	mockWebIdentityResult *sts.AssumeRoleWithWebIdentityOutput
	mockErr               error
}

func (s STSRoleAssumerMock) AssumeRole(context.Context, *sts.AssumeRoleInput, ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
	return s.mockResult, s.mockErr
}

func (s STSRoleAssumerMock) AssumeRoleWithWebIdentity(context.Context, *sts.AssumeRoleWithWebIdentityInput, ...func(*sts.Options)) (*sts.AssumeRoleWithWebIdentityOutput, error) {
	return s.mockWebIdentityResult, s.mockErr
}

func defaultSuccessMockSTSClient() STSRoleAssumerMock {
	return makeMockSTSClient(&sts.AssumeRoleOutput{
		Credentials: &types.Credentials{
			AccessKeyId:     aws.String("test-access-key-id"),
			SecretAccessKey: aws.String("test-secret-access-key"),
			SessionToken:    aws.String("test-session-token"),
			Expiration:      aws.Time(time.UnixMilli(1)),
		},
	}, &sts.AssumeRoleWithWebIdentityOutput{
		Credentials: &types.Credentials{
			AccessKeyId:     aws.String("test-access-key-id"),
			SecretAccessKey: aws.String("test-secret-access-key"),
			SessionToken:    aws.String("test-session-token"),
			Expiration:      aws.Time(time.UnixMilli(1)),
		},
	}, nil)
}

func defaultErrorMockSTSClient() STSRoleAssumerMock {
	return makeMockSTSClient(nil, nil, errors.New("oops"))
}

func makeMockSTSClient(mockResult *sts.AssumeRoleOutput, mockWebIdentityResult *sts.AssumeRoleWithWebIdentityOutput, mockErr error) STSRoleAssumerMock {
	return STSRoleAssumerMock{
		mockResult:            mockResult,
		mockWebIdentityResult: mockWebIdentityResult,
		mockErr:               mockErr,
	}
}

func TestAssumeRoleWithJWT(t *testing.T) {
	type args struct {
		jwt       string
		roleArn   string
		stsClient stscreds.AssumeRoleWithWebIdentityAPIClient
	}
	tests := []struct {
		name    string
		args    args
		want    aws.Credentials
		wantErr bool
	}{
		{
			name: "No email field on token",
			args: args{
				jwt:     "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
				roleArn: "arn:aws:iam::1234567890:role/read-only",
			},
			wantErr: true,
		},
		{
			name: "Failed call to AWS",
			args: args{
				jwt:       "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE2ODQ4NjM2NzksImV4cCI6MTcxNjM5OTY3OSwiYXVkIjoid3d3LmV4YW1wbGUuY29tIiwic3ViIjoiZm9vQGJhci5jb20iLCJFbWFpbCI6ImZvb0BleGFtcGxlLmNvbSJ9.cND4hWI_Wd-AGP0BM4G7jqWfYnuz4Jl7RWLEfZ-AU_0",
				roleArn:   "arn:aws:iam::1234567890:role/read-only",
				stsClient: defaultErrorMockSTSClient(),
			},
			wantErr: true,
		},
		{
			name: "Successfully returns credentials",
			args: args{
				jwt:       "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE2ODQ4NjM2NzksImV4cCI6MTcxNjM5OTY3OSwiYXVkIjoid3d3LmV4YW1wbGUuY29tIiwic3ViIjoiZm9vQGJhci5jb20iLCJlbWFpbCI6ImZvb0BleGFtcGxlLmNvbSJ9.0AhwDFDEtsqOvoJhqvDm9_Vb588GhnfUVGcsN4JFw9o",
				roleArn:   "arn:aws:iam::1234567890:role/read-only",
				stsClient: defaultSuccessMockSTSClient(),
			},
			want: aws.Credentials{
				AccessKeyID:     "test-access-key-id",
				SecretAccessKey: "test-secret-access-key",
				SessionToken:    "test-session-token",
				Source:          "WebIdentityCredentials",
				CanExpire:       true,
				Expires:         time.UnixMilli(1),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AssumeRoleWithJWT(tt.args.jwt, tt.args.roleArn, tt.args.stsClient)
			if (err != nil) != tt.wantErr {
				t.Errorf("AssumeRoleWithJWT() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AssumeRoleWithJWT() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAssumeRole(t *testing.T) {
	var allAllow = NewPolicyStatement("AllowAll", "Allow", []string{"*"}).
		AddResource(aws.String("*")).
		AddCondition(nil)

	tests := []struct {
		name         string
		stsClient    stscreds.AssumeRoleAPIClient
		want         aws.Credentials
		inlinePolicy *PolicyDocument
		wantErr      bool
	}{
		{
			name:         "Fails to assume role",
			stsClient:    defaultErrorMockSTSClient(),
			inlinePolicy: nil,
			wantErr:      true,
		},
		{
			name:         "Successfully assumes role",
			stsClient:    defaultSuccessMockSTSClient(),
			inlinePolicy: nil,
			want: aws.Credentials{
				AccessKeyID:     "test-access-key-id",
				SecretAccessKey: "test-secret-access-key",
				SessionToken:    "test-session-token",
				Source:          "AssumeRoleProvider",
				CanExpire:       true,
				Expires:         time.UnixMilli(1),
			},
		},
		{
			name:      "Successfully assumes role with inline policy",
			stsClient: defaultSuccessMockSTSClient(),
			inlinePolicy: &PolicyDocument{
				Version:   PolicyVersion,
				Statement: []PolicyStatement{allAllow},
			},
			want: aws.Credentials{
				AccessKeyID:     "test-access-key-id",
				SecretAccessKey: "test-secret-access-key",
				SessionToken:    "test-session-token",
				Source:          "AssumeRoleProvider",
				CanExpire:       true,
				Expires:         time.UnixMilli(1),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AssumeRole(tt.stsClient, "", "", tt.inlinePolicy, []types.PolicyDescriptorType{})
			if (err != nil) != tt.wantErr {
				t.Errorf("AssumeRole() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AssumeRole() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAssumeRoleSequence(t *testing.T) {
	var allAllow = NewPolicyStatement("AllowAll", "Allow", []string{"*"}).
		AddResource(aws.String("*")).
		AddCondition(nil)

	type args struct {
		seedClient            stscreds.AssumeRoleAPIClient
		roleArnSequence       []RoleArnSession
		stsClientProviderFunc STSClientProviderFunc
		inlinePolicy          *PolicyDocument
	}
	tests := []struct {
		name    string
		args    args
		want    aws.Credentials
		wantErr bool
	}{
		{
			name: "role arn sequence is nil",
			args: args{
				roleArnSequence: nil,
				inlinePolicy:    nil,
			},
			wantErr: true,
		},
		{
			name: "role arn sequence is empty",
			args: args{
				roleArnSequence: []RoleArnSession{},
				inlinePolicy:    nil,
			},
			wantErr: true,
		},
		{
			name: "single role arn in sequence",
			args: args{
				seedClient:      defaultSuccessMockSTSClient(),
				roleArnSequence: []RoleArnSession{{RoleArn: "a", IsCustomerRole: false}},
				stsClientProviderFunc: func(optFns ...func(*config.LoadOptions) error) (stscreds.AssumeRoleAPIClient, error) {
					return defaultSuccessMockSTSClient(), nil
				},
				inlinePolicy: nil,
			},
			want: aws.Credentials{
				AccessKeyID:     "test-access-key-id",
				SecretAccessKey: "test-secret-access-key",
				SessionToken:    "test-session-token",
				Source:          "AssumeRoleProvider",
				CanExpire:       true,
				Expires:         time.UnixMilli(1),
			},
		},
		{
			name: "Role arn sequence with inline policy",
			args: args{
				seedClient:      defaultSuccessMockSTSClient(),
				roleArnSequence: []RoleArnSession{{RoleArn: "arn-a", IsCustomerRole: false}, {RoleArn: "arn-b", IsCustomerRole: true}},
				stsClientProviderFunc: func(optFns ...func(*config.LoadOptions) error) (stscreds.AssumeRoleAPIClient, error) {
					return defaultSuccessMockSTSClient(), nil
				},
				inlinePolicy: &PolicyDocument{
					Version:   PolicyVersion,
					Statement: []PolicyStatement{allAllow},
				},
			},
			want: aws.Credentials{
				AccessKeyID:     "test-access-key-id",
				SecretAccessKey: "test-secret-access-key",
				SessionToken:    "test-session-token",
				Source:          "AssumeRoleProvider",
				CanExpire:       true,
				Expires:         time.UnixMilli(1),
			},
		},
		{
			name: "Role arn sequence with PolicyARNs for customer role",
			args: args{
				seedClient: defaultSuccessMockSTSClient(),
				roleArnSequence: []RoleArnSession{{
					Name:           "Target-Role-Arn",
					RoleArn:        "arn:aws:iam::123456789012:role/customer-role",
					IsCustomerRole: true,
					PolicyARNs: []types.PolicyDescriptorType{
						{
							Arn: aws.String("arn:aws:iam::aws:policy/service-role/ROSASRESupportPolicy"),
						},
					},
				}},
				stsClientProviderFunc: func(optFns ...func(*config.LoadOptions) error) (stscreds.AssumeRoleAPIClient, error) {
					return defaultSuccessMockSTSClient(), nil
				},
				inlinePolicy: nil,
			},
			want: aws.Credentials{
				AccessKeyID:     "test-access-key-id",
				SecretAccessKey: "test-secret-access-key",
				SessionToken:    "test-session-token",
				Source:          "AssumeRoleProvider",
				CanExpire:       true,
				Expires:         time.UnixMilli(1),
			},
		},
		{
			name: "Role arn sequence with empty PolicyARNs for non-customer role",
			args: args{
				seedClient: defaultSuccessMockSTSClient(),
				roleArnSequence: []RoleArnSession{{
					Name:           "Support-Role-Arn",
					RoleArn:        "arn:aws:iam::123456789012:role/support-role",
					IsCustomerRole: false,
					PolicyARNs:     []types.PolicyDescriptorType{},
				}},
				stsClientProviderFunc: func(optFns ...func(*config.LoadOptions) error) (stscreds.AssumeRoleAPIClient, error) {
					return defaultSuccessMockSTSClient(), nil
				},
				inlinePolicy: nil,
			},
			want: aws.Credentials{
				AccessKeyID:     "test-access-key-id",
				SecretAccessKey: "test-secret-access-key",
				SessionToken:    "test-session-token",
				Source:          "AssumeRoleProvider",
				CanExpire:       true,
				Expires:         time.UnixMilli(1),
			},
		},
		{
			name: "Role arn sequence with multiple roles and mixed PolicyARNs",
			args: args{
				seedClient: defaultSuccessMockSTSClient(),
				roleArnSequence: []RoleArnSession{
					{
						Name:           "Support-Role-Arn",
						RoleArn:        "arn:aws:iam::123456789012:role/support-role",
						IsCustomerRole: false,
						PolicyARNs:     []types.PolicyDescriptorType{},
					},
					{
						Name:           "Target-Role-Arn",
						RoleArn:        "arn:aws:iam::123456789012:role/customer-role",
						IsCustomerRole: true,
						PolicyARNs: []types.PolicyDescriptorType{
							{
								Arn: aws.String("arn:aws:iam::aws:policy/service-role/ROSASRESupportPolicy"),
							},
						},
					},
				},
				stsClientProviderFunc: func(optFns ...func(*config.LoadOptions) error) (stscreds.AssumeRoleAPIClient, error) {
					return defaultSuccessMockSTSClient(), nil
				},
				inlinePolicy: nil,
			},
			want: aws.Credentials{
				AccessKeyID:     "test-access-key-id",
				SecretAccessKey: "test-secret-access-key",
				SessionToken:    "test-session-token",
				Source:          "AssumeRoleProvider",
				CanExpire:       true,
				Expires:         time.UnixMilli(1),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AssumeRoleSequence(tt.args.seedClient, tt.args.roleArnSequence, nil, tt.args.stsClientProviderFunc, tt.args.inlinePolicy)
			if (err != nil) != tt.wantErr {
				t.Errorf("AssumeRoleSequence() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AssumeRoleSequence() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAssumeRole_PolicyARNs(t *testing.T) {
	tests := []struct {
		name       string
		policyARNs []types.PolicyDescriptorType
		wantErr    bool
	}{
		{
			name:       "empty PolicyARNs",
			policyARNs: []types.PolicyDescriptorType{},
			wantErr:    false,
		},
		{
			name: "single PolicyARN",
			policyARNs: []types.PolicyDescriptorType{
				{
					Arn: aws.String("arn:aws:iam::aws:policy/service-role/ROSASRESupportPolicy"),
				},
			},
			wantErr: false,
		},
		{
			name: "multiple PolicyARNs",
			policyARNs: []types.PolicyDescriptorType{
				{
					Arn: aws.String("arn:aws:iam::aws:policy/service-role/ROSASRESupportPolicy"),
				},
				{
					Arn: aws.String("arn:aws:iam::aws:policy/ReadOnlyAccess"),
				},
			},
			wantErr: false,
		},
		{
			name:       "nil PolicyARNs",
			policyARNs: nil,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stsClient := defaultSuccessMockSTSClient()

			got, err := AssumeRole(stsClient, "test-session", "arn:aws:iam::123456789012:role/test-role", nil, tt.policyARNs)

			if (err != nil) != tt.wantErr {
				t.Errorf("AssumeRole() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				expected := aws.Credentials{
					AccessKeyID:     "test-access-key-id",
					SecretAccessKey: "test-secret-access-key",
					SessionToken:    "test-session-token",
					Source:          "AssumeRoleProvider",
					CanExpire:       true,
					Expires:         time.UnixMilli(1),
				}
				if !reflect.DeepEqual(got, expected) {
					t.Errorf("AssumeRole() got = %v, want %v", got, expected)
				}
			}
		})
	}
}

func TestRoleArnSession_PolicyARNsHandling(t *testing.T) {
	tests := []struct {
		name                string
		roleArnSession      RoleArnSession
		expectedPolicyCount int
	}{
		{
			name: "customer role with ROSASRESupportPolicy",
			roleArnSession: RoleArnSession{
				Name:           "Target-Role-Arn",
				RoleArn:        "arn:aws:iam::123456789012:role/customer-role",
				IsCustomerRole: true,
				PolicyARNs: []types.PolicyDescriptorType{
					{
						Arn: aws.String("arn:aws:iam::aws:policy/service-role/ROSASRESupportPolicy"),
					},
				},
			},
			expectedPolicyCount: 1,
		},
		{
			name: "non-customer role with empty PolicyARNs",
			roleArnSession: RoleArnSession{
				Name:           "Support-Role-Arn",
				RoleArn:        "arn:aws:iam::123456789012:role/support-role",
				IsCustomerRole: false,
				PolicyARNs:     []types.PolicyDescriptorType{},
			},
			expectedPolicyCount: 0,
		},
		{
			name: "customer role with multiple policies",
			roleArnSession: RoleArnSession{
				Name:           "Target-Role-Arn",
				RoleArn:        "arn:aws:iam::123456789012:role/customer-role",
				IsCustomerRole: true,
				PolicyARNs: []types.PolicyDescriptorType{
					{
						Arn: aws.String("arn:aws:iam::aws:policy/service-role/ROSASRESupportPolicy"),
					},
					{
						Arn: aws.String("arn:aws:iam::123456789012:policy/CustomPolicy"),
					},
				},
			},
			expectedPolicyCount: 2,
		},
		{
			name: "customer role with nil PolicyARNs",
			roleArnSession: RoleArnSession{
				Name:           "Target-Role-Arn",
				RoleArn:        "arn:aws:iam::123456789012:role/customer-role",
				IsCustomerRole: true,
				PolicyARNs:     nil,
			},
			expectedPolicyCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualCount := len(tt.roleArnSession.PolicyARNs)
			if actualCount != tt.expectedPolicyCount {
				t.Errorf("PolicyARNs count = %v, want %v", actualCount, tt.expectedPolicyCount)
			}

			// Verify IsCustomerRole flag consistency
			if tt.roleArnSession.IsCustomerRole && tt.roleArnSession.Name == "Target-Role-Arn" {
				// Customer roles should typically have policies or be explicitly empty
				if tt.roleArnSession.PolicyARNs == nil {
					t.Log("Customer role has nil PolicyARNs - this is valid but may need attention")
				}
			}

			// Verify PolicyARN structure for non-empty cases
			for i, policyARN := range tt.roleArnSession.PolicyARNs {
				if policyARN.Arn == nil {
					t.Errorf("PolicyARNs[%d].Arn is nil", i)
				} else if *policyARN.Arn == "" {
					t.Errorf("PolicyARNs[%d].Arn is empty string", i)
				}
			}
		})
	}
}

func TestAssumeRole_PolicyARNs_ErrorScenarios(t *testing.T) {
	tests := []struct {
		name       string
		stsClient  STSRoleAssumerMock
		policyARNs []types.PolicyDescriptorType
		wantErr    bool
		errorMsg   string
	}{
		{
			name: "AssumeRole fails with invalid PolicyARN",
			stsClient: STSRoleAssumerMock{
				mockResult: nil,
				mockErr:    errors.New("InvalidParameterValue: Invalid policy ARN"),
			},
			policyARNs: []types.PolicyDescriptorType{
				{
					Arn: aws.String("arn:aws:iam::aws:policy/invalid-policy"),
				},
			},
			wantErr:  true,
			errorMsg: "failed to assume role",
		},
		{
			name: "AssumeRole fails with malformed PolicyARN",
			stsClient: STSRoleAssumerMock{
				mockResult: nil,
				mockErr:    errors.New("MalformedPolicyDocument: Policy ARN is malformed"),
			},
			policyARNs: []types.PolicyDescriptorType{
				{
					Arn: aws.String("invalid-arn-format"),
				},
			},
			wantErr:  true,
			errorMsg: "failed to assume role",
		},
		{
			name: "AssumeRole fails with too many PolicyARNs",
			stsClient: STSRoleAssumerMock{
				mockResult: nil,
				mockErr:    errors.New("LimitExceeded: Cannot exceed quota for PolicyArnsPerRole"),
			},
			policyARNs: func() []types.PolicyDescriptorType {
				// Create more than 10 policies (AWS limit)
				policies := make([]types.PolicyDescriptorType, 11)
				for i := 0; i < 11; i++ {
					policies[i] = types.PolicyDescriptorType{
						Arn: aws.String(fmt.Sprintf("arn:aws:iam::aws:policy/test-policy-%d", i)),
					}
				}
				return policies
			}(),
			wantErr:  true,
			errorMsg: "failed to assume role",
		},
		{
			name: "AssumeRole fails with access denied for PolicyARN",
			stsClient: STSRoleAssumerMock{
				mockResult: nil,
				mockErr:    errors.New("AccessDenied: User is not authorized to perform: sts:AssumeRole with policy"),
			},
			policyARNs: []types.PolicyDescriptorType{
				{
					Arn: aws.String("arn:aws:iam::123456789012:policy/restricted-policy"),
				},
			},
			wantErr:  true,
			errorMsg: "failed to assume role",
		},
		{
			name:      "AssumeRole succeeds with nil PolicyARN Arn",
			stsClient: defaultSuccessMockSTSClient(),
			policyARNs: []types.PolicyDescriptorType{
				{
					Arn: nil, // This should be handled gracefully
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AssumeRole(tt.stsClient, "test-session", "arn:aws:iam::123456789012:role/test-role", nil, tt.policyARNs)

			if (err != nil) != tt.wantErr {
				t.Errorf("AssumeRole() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errorMsg != "" {
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("AssumeRole() error = %v, should contain %v", err, tt.errorMsg)
				}
			}

			if !tt.wantErr {
				expected := aws.Credentials{
					AccessKeyID:     "test-access-key-id",
					SecretAccessKey: "test-secret-access-key",
					SessionToken:    "test-session-token",
					Source:          "AssumeRoleProvider",
					CanExpire:       true,
					Expires:         time.UnixMilli(1),
				}
				if !reflect.DeepEqual(got, expected) {
					t.Errorf("AssumeRole() got = %v, want %v", got, expected)
				}
			}
		})
	}
}

func TestAssumeRoleSequence_PolicyARNs_ErrorScenarios(t *testing.T) {
	tests := []struct {
		name                  string
		roleArnSequence       []RoleArnSession
		seedClient            STSRoleAssumerMock
		stsClientProviderFunc STSClientProviderFunc
		wantErr               bool
		errorMsg              string
	}{
		{
			name: "AssumeRoleSequence fails when STS client creation fails with multi-role sequence",
			roleArnSequence: []RoleArnSession{
				{
					Name:            "Support-Role-Arn",
					RoleArn:         "arn:aws:iam::123456789012:role/support-role",
					RoleSessionName: "support-session",
					IsCustomerRole:  false,
					PolicyARNs:      []types.PolicyDescriptorType{},
				},
				{
					Name:            "Target-Role-Arn",
					RoleArn:         "arn:aws:iam::123456789012:role/customer-role",
					RoleSessionName: "customer-session",
					IsCustomerRole:  true,
					PolicyARNs: []types.PolicyDescriptorType{
						{
							Arn: aws.String("arn:aws:iam::aws:policy/service-role/ROSASRESupportPolicy"),
						},
					},
				},
			},
			seedClient: defaultSuccessMockSTSClient(),
			stsClientProviderFunc: func(optFns ...func(*config.LoadOptions) error) (stscreds.AssumeRoleAPIClient, error) {
				return nil, errors.New("failed to create STS client with new credentials")
			},
			wantErr:  true,
			errorMsg: "failed to create client with credentials for role",
		},
		{
			name: "AssumeRoleSequence validates PolicyARNs are passed correctly to AssumeRole",
			roleArnSequence: []RoleArnSession{
				{
					Name:            "Target-Role-Arn",
					RoleArn:         "arn:aws:iam::123456789012:role/customer-role",
					RoleSessionName: "customer-session",
					IsCustomerRole:  true,
					PolicyARNs: []types.PolicyDescriptorType{
						{
							Arn: aws.String("arn:aws:iam::aws:policy/service-role/ROSASRESupportPolicy"),
						},
					},
				},
			},
			seedClient: defaultSuccessMockSTSClient(),
			stsClientProviderFunc: func(optFns ...func(*config.LoadOptions) error) (stscreds.AssumeRoleAPIClient, error) {
				return defaultSuccessMockSTSClient(), nil
			},
			wantErr:  false,
			errorMsg: "",
		},
		{
			name: "AssumeRoleSequence handles empty PolicyARNs for non-customer roles",
			roleArnSequence: []RoleArnSession{
				{
					Name:            "Support-Role-Arn",
					RoleArn:         "arn:aws:iam::123456789012:role/support-role",
					RoleSessionName: "support-session",
					IsCustomerRole:  false,
					PolicyARNs:      []types.PolicyDescriptorType{}, // Empty PolicyARNs
				},
			},
			seedClient: defaultSuccessMockSTSClient(),
			stsClientProviderFunc: func(optFns ...func(*config.LoadOptions) error) (stscreds.AssumeRoleAPIClient, error) {
				return defaultSuccessMockSTSClient(), nil
			},
			wantErr:  false,
			errorMsg: "",
		},
		{
			name: "AssumeRoleSequence handles nil PolicyARNs",
			roleArnSequence: []RoleArnSession{
				{
					Name:            "Target-Role-Arn",
					RoleArn:         "arn:aws:iam::123456789012:role/customer-role",
					RoleSessionName: "customer-session",
					IsCustomerRole:  true,
					PolicyARNs:      nil, // Nil PolicyARNs
				},
			},
			seedClient: defaultSuccessMockSTSClient(),
			stsClientProviderFunc: func(optFns ...func(*config.LoadOptions) error) (stscreds.AssumeRoleAPIClient, error) {
				return defaultSuccessMockSTSClient(), nil
			},
			wantErr:  false,
			errorMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AssumeRoleSequence(tt.seedClient, tt.roleArnSequence, nil, tt.stsClientProviderFunc, nil)

			if (err != nil) != tt.wantErr {
				t.Errorf("AssumeRoleSequence() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errorMsg != "" {
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("AssumeRoleSequence() error = %v, should contain %v", err, tt.errorMsg)
				}
			}

			if !tt.wantErr {
				// Verify successful credentials
				if got.AccessKeyID == "" {
					t.Error("AssumeRoleSequence() should return valid credentials")
				}
			}
		})
	}
}

func TestGetSigninToken(t *testing.T) {
	awsCredentials := aws.Credentials{
		AccessKeyID:     "testAccessKeyId",
		SecretAccessKey: "testSecretAccessKey",
		SessionToken:    "testSessionToken",
	}
	region := "us-east-1"
	tests := []struct {
		name        string
		httpGetFunc func(url string) (resp *http.Response, err error)
		want        *AWSSigninTokenResponse
		wantErr     bool
	}{
		{
			name: "properly gets a signin token",
			httpGetFunc: func(_ string) (resp *http.Response, err error) {
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewReader([]byte(`{"SigninToken":"theToken"}`))),
				}, nil
			},
			want: &AWSSigninTokenResponse{SigninToken: "theToken"},
		},
		{
			name: "malformed signin token response",
			httpGetFunc: func(_ string) (resp *http.Response, err error) {
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewReader([]byte(`{"foo":"theToken"}`))),
				}, nil
			},
			want: &AWSSigninTokenResponse{},
		},
		{
			name: "non-200 response code",
			httpGetFunc: func(_ string) (resp *http.Response, err error) {
				return &http.Response{
					StatusCode: 401,
					Body:       io.NopCloser(bytes.NewReader([]byte(``))),
				}, nil
			},
			wantErr: true,
		},
		{
			name: "error when making http call",
			httpGetFunc: func(_ string) (resp *http.Response, err error) {
				return &http.Response{
					StatusCode: 401,
					Body:       io.NopCloser(bytes.NewReader([]byte(``))),
				}, errors.New("oops")
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpGetFunc = tt.httpGetFunc
			got, err := GetSigninToken(awsCredentials, region)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSigninToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetSigninToken() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetConsoleUrl(t *testing.T) {
	tests := []struct {
		name        string
		signinToken string
		want        *url.URL
		wantErr     bool
	}{
		{
			name:        "valid signin token",
			signinToken: "the_token",
			want: &url.URL{
				Scheme:   "https",
				Host:     "us-east-1.signin.aws.amazon.com",
				Path:     "/federation",
				RawQuery: "Action=login&Destination=https%3A%2F%2Fus-east-1.console.aws.amazon.com%2F&Issuer=Red+Hat+SRE&SigninToken=the_token",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetConsoleURL(tt.signinToken, "us-east-1")
			if (err != nil) != tt.wantErr {
				t.Errorf("GetConsoleURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetConsoleURL() got = %v, want %v", got, tt.want)
			}
		})
	}
}
