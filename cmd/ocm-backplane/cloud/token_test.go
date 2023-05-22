package cloud

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
	"reflect"
	"testing"
)

type STSRoleAssumerMock struct {
	mockResult *sts.AssumeRoleWithWebIdentityOutput
	mockErr    error
}

func (s STSRoleAssumerMock) AssumeRoleWithWebIdentity(context.Context, *sts.AssumeRoleWithWebIdentityInput, ...func(*sts.Options)) (*sts.AssumeRoleWithWebIdentityOutput, error) {
	return s.mockResult, s.mockErr
}

func makeMockSTSClient(mockResult *sts.AssumeRoleWithWebIdentityOutput, mockErr error) STSRoleAssumerMock {
	return STSRoleAssumerMock{
		mockResult: mockResult,
		mockErr:    mockErr,
	}
}

func TestGetStsToken(t *testing.T) {
	type args struct {
		ocmToken string
		roleArn  string
		svc      STSRoleAssumer
	}
	tests := []struct {
		name    string
		args    args
		want    *types.Credentials
		wantErr bool
	}{
		{
			name: "No email field on token",
			args: args{
				ocmToken: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
				roleArn:  "arn:aws:iam::1234567890:role/read-only",
			},
			wantErr: true,
		},
		{
			name: "Failed call to AWS",
			args: args{
				ocmToken: "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE2ODQ4NjM2NzksImV4cCI6MTcxNjM5OTY3OSwiYXVkIjoid3d3LmV4YW1wbGUuY29tIiwic3ViIjoiZm9vQGJhci5jb20iLCJFbWFpbCI6ImZvb0BleGFtcGxlLmNvbSJ9.cND4hWI_Wd-AGP0BM4G7jqWfYnuz4Jl7RWLEfZ-AU_0",
				roleArn:  "arn:aws:iam::1234567890:role/read-only",
				svc:      makeMockSTSClient(nil, errors.New("oops")),
			},
			wantErr: true,
		},
		{
			name: "Successfully returns credentials",
			args: args{
				ocmToken: "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE2ODQ4NjM2NzksImV4cCI6MTcxNjM5OTY3OSwiYXVkIjoid3d3LmV4YW1wbGUuY29tIiwic3ViIjoiZm9vQGJhci5jb20iLCJlbWFpbCI6ImZvb0BleGFtcGxlLmNvbSJ9.0AhwDFDEtsqOvoJhqvDm9_Vb588GhnfUVGcsN4JFw9o",
				roleArn:  "arn:aws:iam::1234567890:role/read-only",
				svc: makeMockSTSClient(&sts.AssumeRoleWithWebIdentityOutput{
					AssumedRoleUser: &types.AssumedRoleUser{
						Arn:           aws.String("arn:aws:sts::123456789:assumed-role/read-only/foo@example.com"),
						AssumedRoleId: aws.String("ABCDEFG1234567:foo@example.com"),
					},
					Audience: aws.String("sample-audience"),
					Credentials: &types.Credentials{
						AccessKeyId:     aws.String("test-access-key-id"),
						SecretAccessKey: aws.String("test-secret-access-key"),
						SessionToken:    aws.String("test-session-token"),
					},
					Provider:                    aws.String("arn:aws:iam::123456789:oidc-provider/foo.example.com/auth/"),
					SubjectFromWebIdentityToken: aws.String("f:123abc-45de-67fg-8hi9-abcde12345:foo"),
				}, nil),
			},
			want: &types.Credentials{
				AccessKeyId:     aws.String("test-access-key-id"),
				SecretAccessKey: aws.String("test-secret-access-key"),
				SessionToken:    aws.String("test-session-token"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetStsCredentials(tt.args.ocmToken, tt.args.roleArn, tt.args.svc)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetStsCredentials() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetStsCredentials() got = %v, want %v", got, tt.want)
			}
		})
	}
}
