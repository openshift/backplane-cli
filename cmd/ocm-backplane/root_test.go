package main

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Backplane commands", func() {
	Context("Test root cmd", func() {

		It("Check root cmd help ", func() {
			err := rootCmd.Help()
			Expect(err).To(BeNil())
		})

		It("Check verbosity persistent flag", func() {
			flagSet := rootCmd.PersistentFlags()
			verbosityFlag := flagSet.Lookup("verbosity")
			Expect(verbosityFlag).NotTo(BeNil())

			// check the deafult log level
			Expect(verbosityFlag.DefValue).To(Equal("warning"))

		})
	})
})
