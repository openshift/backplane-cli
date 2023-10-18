package utils

import "os"

var ShellChecker ShellCheckerInterface = DefaultShellChecker{}

type ShellCheckerInterface interface {
	IsValidShell(shellPath string) bool
}

type DefaultShellChecker struct{}

// Helper function to check if a shell is valid
func (checker DefaultShellChecker) IsValidShell(shellPath string) bool {
	_, err := os.Stat(shellPath)
	return err == nil
}
