package upgrade

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("validateReleaseVersion", func() {
	Context("when the release version metadata is missing", func() {
		It("returns a clear, actionable error", func() {
			err := validateReleaseVersion("")

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("release version metadata is missing"))
			Expect(err.Error()).To(ContainSubstring("built locally from source"))
			Expect(err.Error()).To(ContainSubstring("backplane-tools upgrade backplane-cli"))
		})

		It("does not expose the raw SemVer parsing error", func() {
			err := validateReleaseVersion("")

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).ToNot(ContainSubstring("Invalid Semantic Version"))
		})
	})

	Context("when the release version metadata is present", func() {
		It("does not trigger the missing-version validation", func() {
			Expect(validateReleaseVersion("1.0.1")).To(Succeed())
		})

		It("passes the version through unchanged regardless of its format", func() {
			Expect(validateReleaseVersion("1.0.1-0.20261009184111-fc8c9e8dedd0")).To(Succeed())
		})
	})
})
