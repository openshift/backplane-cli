package utils

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ShellChecker", func() {
	var (
		shellChecker ShellCheckerInterface
	)

	BeforeEach(func() {
		shellChecker = DefaultShellChecker{}
	})

	Context("When checking a valid shell", func() {
		It("Should return true for a valid shell path", func() {
			validShellPath := "/bin/bash"
			Expect(shellChecker.IsValidShell(validShellPath)).To(BeTrue())
		})

		It("Should return true for another valid shell path", func() {
			validShellPath := "/bin/zsh"
			Expect(shellChecker.IsValidShell(validShellPath)).To(BeTrue())
		})
	})

	Context("When checking an invalid shell", func() {
		It("Should return false for an invalid shell path", func() {
			invalidShellPath := "/invalid/shell/path"
			Expect(shellChecker.IsValidShell(invalidShellPath)).To(BeFalse())
		})

		It("Should return false for another invalid shell path", func() {
			invalidShellPath := "/another/invalid/shell/path"
			Expect(shellChecker.IsValidShell(invalidShellPath)).To(BeFalse())
		})
	})

	Context("When checking an empty shell path", func() {
		It("Should return false", func() {
			Expect(shellChecker.IsValidShell("")).To(BeFalse())
		})
	})

	Context("When checking a non-existent shell path", func() {
		It("Should return false", func() {
			nonExistentShellPath := "/path/that/does/not/exist"
			Expect(shellChecker.IsValidShell(nonExistentShellPath)).To(BeFalse())
		})
	})
})
