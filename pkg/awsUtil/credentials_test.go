package awsUtil

import (
	"encoding/json"
	"fmt"
	"sigs.k8s.io/yaml"
	"testing"
)

func TestAWSCredentialsResponse_EnvFormat(t *testing.T) {
	type fields struct {
		AccessKeyId     string
		SecretAccessKey string
		SessionToken    string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "Contains no values",
			want: fmt.Sprintf(awsExportFormat, "", "", ""),
		},
		{
			name:   "Contains Access Key Id only",
			fields: fields{AccessKeyId: "test-key"},
			want:   fmt.Sprintf(awsExportFormat, "test-key", "", ""),
		},
		{
			name:   "Contains Secret Access Key only",
			fields: fields{SecretAccessKey: "test-secret-key"},
			want:   fmt.Sprintf(awsExportFormat, "", "test-secret-key", ""),
		},
		{
			name:   "Contains Session Token only",
			fields: fields{SessionToken: "test-session-token"},
			want:   fmt.Sprintf(awsExportFormat, "", "", "test-session-token"),
		},
		{
			name:   "Contains Access Key Id and Secret Access Key",
			fields: fields{AccessKeyId: "test-key", SecretAccessKey: "test-secret-key"},
			want:   fmt.Sprintf(awsExportFormat, "test-key", "test-secret-key", ""),
		},
		{
			name:   "Contains Access Key Id and Session Token",
			fields: fields{AccessKeyId: "test-key", SessionToken: "test-session-token"},
			want:   fmt.Sprintf(awsExportFormat, "test-key", "", "test-session-token"),
		},
		{
			name:   "Contains Secret Access Key and Session Token",
			fields: fields{SecretAccessKey: "test-secret-key", SessionToken: "test-session-token"},
			want:   fmt.Sprintf(awsExportFormat, "", "test-secret-key", "test-session-token"),
		},
		{
			name:   "Contains Access Key Id, Secret Access Key, and Session Token",
			fields: fields{AccessKeyId: "test-key", SecretAccessKey: "test-secret-key", SessionToken: "test-session-token"},
			want:   fmt.Sprintf(awsExportFormat, "test-key", "test-secret-key", "test-session-token"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := AWSCredentialsResponse{
				AccessKeyId:     tt.fields.AccessKeyId,
				SecretAccessKey: tt.fields.SecretAccessKey,
				SessionToken:    tt.fields.SessionToken,
			}
			if got := r.EnvFormat(); got != tt.want {
				t.Errorf("EnvFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAWSCredentialsResponse_RenderOutput(t *testing.T) {
	type fields struct {
		AccessKeyId     string
		SecretAccessKey string
		SessionToken    string
		Region          string
		Expiration      string
	}
	type args struct {
		outputFormat string
	}
	credentials := fields{
		AccessKeyId:     "1",
		SecretAccessKey: "2",
		SessionToken:    "3",
		Region:          "4",
		Expiration:      "5",
	}
	jsonOutput, _ := json.Marshal(credentials)
	yamlOutput, _ := yaml.Marshal(credentials)

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr bool
	}{
		{
			"Render JSON", credentials, args{
				outputFormat: "json",
			},
			string(jsonOutput),
			false,
		},
		{
			"Render YAML",
			credentials,
			args{
				outputFormat: "yaml",
			},
			string(yamlOutput),
			false,
		},
		{
			"Render Invalid",
			credentials,
			args{
				outputFormat: "invalid",
			},
			"",
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := AWSCredentialsResponse{
				AccessKeyId:     tt.fields.AccessKeyId,
				SecretAccessKey: tt.fields.SecretAccessKey,
				SessionToken:    tt.fields.SessionToken,
				Region:          tt.fields.Region,
				Expiration:      tt.fields.Expiration,
			}
			got, err := r.RenderOutput(tt.args.outputFormat)
			if (err != nil) != tt.wantErr {
				t.Errorf("AWSCredentialsResponse.RenderOutput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("AWSCredentialsResponse.RenderOutput() = %v, want %v", got, tt.want)
			}
		})
	}
}
