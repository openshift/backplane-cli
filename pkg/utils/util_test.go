package utils

import (
	"fmt"
	"reflect"
	"testing"
)

func TestParseParamFlag(t *testing.T) {
	tests := []struct {
		inp    []string
		expect map[string]string
		expErr bool
	}{
		{
			inp:    []string{"k1=v1"},
			expect: map[string]string{"k1": "v1"},
			expErr: false,
		},
		{
			inp:    []string{"k1=v1", "k2=v2"},
			expect: map[string]string{"k1": "v1", "k2": "v2"},
			expErr: false,
		},
		{
			inp:    []string{"k1=v1", "k1=v2"},
			expect: map[string]string{"k1": "v2"},
			expErr: false,
		},
		{
			inp:    []string{"k1"},
			expect: nil,
			expErr: true,
		},
		{
			inp:    []string{"k1="},
			expect: map[string]string{"k1": ""},
			expErr: false,
		},
	}

	for n, tt := range tests {
		t.Run(fmt.Sprintf("case %d", n), func(t *testing.T) {
			result, err := ParseParamsFlag(tt.inp)
			if !reflect.DeepEqual(result, tt.expect) {
				t.Errorf("Expecting: %s, but get: %s", tt.expect, result)
			}
			if tt.expErr && err == nil {
				t.Errorf("Expecting error but got none")
			}
		})
	}
}

func TestGetFreePort(t *testing.T) {
	port, err := GetFreePort()
	if err != nil {
		t.Errorf("unable get port")
	}
	if port <= 1024 || port > 65535 {
		t.Errorf("unexpected port %d", port)
	}
}

func TestMatchBaseDomain(t *testing.T) {
	tests := []struct {
		name       string
		longURL    string
		baseDomain string
		expect     bool
	}{
		{
			name:       "case-1",
			longURL:    "a.example.com",
			baseDomain: "example.com",
			expect:     true,
		},
		{
			name:       "case-2",
			longURL:    "a.b.c.example.com",
			baseDomain: "example.com",
			expect:     true,
		},
		{
			name:       "case-3",
			longURL:    "example.com",
			baseDomain: "example.com",
			expect:     true,
		},
		{
			name:       "case-4",
			longURL:    "a.example.com",
			baseDomain: "",
			expect:     true,
		},
		{
			name:       "case-5",
			longURL:    "",
			baseDomain: "",
			expect:     true,
		},
		{
			name:       "case-6",
			longURL:    "",
			baseDomain: "example.com",
			expect:     false,
		},
		{
			name:       "case-7",
			longURL:    "a.example.com.io",
			baseDomain: "example.com",
			expect:     false,
		},
		{
			name:       "case-8",
			longURL:    "a.b.c",
			baseDomain: "e.f.g",
			expect:     false,
		},
		{
			name:       "case-9",
			longURL:    "a",
			baseDomain: "a",
			expect:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchBaseDomain(tt.longURL, tt.baseDomain)
			if result != tt.expect {
				t.Errorf("Expecting: %t, but get: %t", tt.expect, result)
			}
		})
	}
}
