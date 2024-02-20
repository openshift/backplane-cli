package awsutil

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
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
	tests := []struct {
		name      string
		stsClient stscreds.AssumeRoleAPIClient
		want      aws.Credentials
		wantErr   bool
	}{
		{
			name:      "Fails to assume role",
			stsClient: defaultErrorMockSTSClient(),
			wantErr:   true,
		},
		{
			name:      "Successfully assumes role",
			stsClient: defaultSuccessMockSTSClient(),
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
			got, err := AssumeRole(tt.stsClient, "", "")
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
	type args struct {
		seedClient            stscreds.AssumeRoleAPIClient
		roleArnSequence       []RoleArnSession
		stsClientProviderFunc STSClientProviderFunc
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
			},
			wantErr: true,
		},
		{
			name: "role arn sequence is empty",
			args: args{
				roleArnSequence: []RoleArnSession{},
			},
			wantErr: true,
		},
		{
			name: "single role arn in sequence",
			args: args{
				seedClient:      defaultSuccessMockSTSClient(),
				roleArnSequence: []RoleArnSession{{RoleArn: "a"}},
				stsClientProviderFunc: func(optFns ...func(*config.LoadOptions) error) (stscreds.AssumeRoleAPIClient, error) {
					return defaultSuccessMockSTSClient(), nil
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AssumeRoleSequence(tt.args.seedClient, tt.args.roleArnSequence, nil, tt.args.stsClientProviderFunc)
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
