package utils

import (
	"testing"
)

func TestGetFieldFromJWT(t *testing.T) {
	type testCase struct {
		name    string
		token   string
		field   string
		want    string
		wantErr bool
	}
	tests := []testCase{
		{
			name:  "Get string field",
			token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			field: "sub",
			want:  "1234567890",
		},
		{
			name:    "Get number field",
			token:   "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjAsImV4cCI6MTcxNjY1MDA3MSwiYXVkIjoid3d3LmV4YW1wbGUuY29tIiwic3ViIjoianJvY2tldEBleGFtcGxlLmNvbSJ9._CyJxncO4NBOH6a-Q_2oIVelCRZKJh9YiPBm4XEBZgI",
			field:   "iat",
			wantErr: true,
		},
		{
			name:    "Get field that doesn't exist",
			token:   "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			field:   "foo",
			wantErr: true,
		},
		{
			name:    "Invalid token",
			token:   "abcdefg",
			field:   "foo",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetStringFieldFromJWT(tt.token, tt.field)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetStringFieldFromJWT() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetStringFieldFromJWT() got = %v, want %v", got, tt.want)
			}
		})
	}
}
