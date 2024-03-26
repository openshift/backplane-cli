package utils

import (
	"testing"
)

// MockShellChecker is a mock implementation of ShellCheckerInterface for testing purposes.
type MockShellChecker struct{}

// IsValidShell is a mock implementation of IsValidShell method.
func (m MockShellChecker) IsValidShell(shellPath string) bool {
	// For testing purposes, always return true.
	return true
}

func TestIsValidShell(t *testing.T) {
	// Create an instance of the DefaultShellChecker.
	checker := DefaultShellChecker{}

	// Define test cases.
	tests := []struct {
		name      string
		shellpath string
		expect    bool
	}{
		{
			name:      "case 1",
			shellpath: "/bin/sh",
			expect:    true,
		},
		{
			name:      "case 2",
			shellpath: "/bin/csh",
			expect:    true,
		},
		{
			name:      "case 3",
			shellpath: "/bin/ksh",
			expect:    true,
		},
		{
			name:      "case 4",
			shellpath: "/bin/bash",
			expect:    true,
		},
		{
			name:      "case 5",
			shellpath: "/bin/tcsh",
			expect:    true,
		},
		{
			name:      "case 6",
			shellpath: "/bin/zsh",
			expect:    true,
		},
		{
			name:      "case 7",
			shellpath: "/usr/bin/fish",
			expect:    true,
		},
		{
			name:      "case 8",
			shellpath: "/bin/dash",
			expect:    true,
		},
	}

	// Run tests.
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := checker.IsValidShell(test.shellpath)
			if result != test.expect {
				t.Errorf("Expected IsValidShell(%s) to return %t, but got %t", test.shellpath, test.expect, result)
			}
		})
	}
}
