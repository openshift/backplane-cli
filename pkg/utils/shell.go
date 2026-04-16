package utils

import "os"

var ShellChecker ShellCheckerInterface = DefaultShellChecker{}

//go:generate go tool mockgen -destination=mocks/shellCheckerMock.go -package=mocks github.com/openshift/backplane-cli/pkg/utils ShellCheckerInterface
type ShellCheckerInterface interface {
	IsValidShell(shellPath string) bool
}

type DefaultShellChecker struct{}

// Helper function to check if a shell is valid
func (checker DefaultShellChecker) IsValidShell(shellPath string) bool {
	_, err := os.Stat(shellPath)
	return err == nil
}
