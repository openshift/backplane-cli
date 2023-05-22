package utils

import (
	"testing"
)

func TestGetFieldFromJWT(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		field   string
		want    string
		wantErr bool
	}{
		{
			name:    "Get field",
			token:   "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			field:   "sub",
			want:    "1234567890",
			wantErr: false,
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
			got, err := GetFieldFromJWT(tt.token, tt.field)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetFieldFromJWT() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetFieldFromJWT() got = %v, want %v", got, tt.want)
			}
		})
	}
}
