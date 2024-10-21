package backplaneapi_test

import (
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/openshift/backplane-cli/pkg/backplaneapi"
)

var _ = Describe("backplaneapi/deprecation", func() {
	It("Returns ErrDeprecation when the 'Deprecated-Client' header is present.", func() {
		r := http.Response{Header: http.Header{}}

		r.Header.Add("Deprecated-Client", "true")

		err := backplaneapi.CheckResponseDeprecation(&r)

		Expect(err).To(Equal(backplaneapi.ErrDeprecation))
	})

	It("Returns nil when the 'Deprecated-Client' header is not present.", func() {
		r := http.Response{Header: http.Header{}}

		err := backplaneapi.CheckResponseDeprecation(&r)

		Expect(err).To(BeNil())
	})

	It("Returns nil when the 'Deprecated-Client' header is not 'true'.", func() {
		r := http.Response{Header: http.Header{}}

		r.Header.Add("Deprecated-Client", "false")

		err := backplaneapi.CheckResponseDeprecation(&r)

		Expect(err).To(BeNil())
	})
})
