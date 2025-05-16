package ocm_test

import (
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ocmsdk "github.com/openshift-online/ocm-sdk-go"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift/backplane-cli/pkg/ocm/mocks"
)

var _ = Describe("OCM Wrapper Test", func() {
	var (
		ctrl             *gomock.Controller
		mockOcmInterface *mocks.MockOCMInterface
		ocmConnection    *ocmsdk.Connection
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT()) // Initialize the controller
		mockOcmInterface = mocks.NewMockOCMInterface(ctrl)
		ocmConnection = &ocmsdk.Connection{}
	})

	AfterEach(func() {
		ctrl.Finish() // Ensures that all expectations were met
	})

	Context("Test OCM Wrapper", func() {
		It("Should return trusted IPList", func() {
			ip1 := cmv1.NewTrustedIp().ID("100.10.10.10")
			ip2 := cmv1.NewTrustedIp().ID("200.20.20.20")
			expectedIPList, _ := cmv1.NewTrustedIpList().Items(ip1, ip2).Build()
			mockOcmInterface.EXPECT().GetTrustedIPList(gomock.Any()).Return(expectedIPList, nil).AnyTimes()

			IPList, err := mockOcmInterface.GetTrustedIPList(ocmConnection)
			Expect(err).To(BeNil())
			Expect(len(IPList.Items())).Should(Equal(2))
		})

		It("Should not return errors for empty trusted IPList", func() {
			mockOcmInterface.EXPECT().GetTrustedIPList(gomock.Any()).Return(nil, nil).AnyTimes()
			_, err := mockOcmInterface.GetTrustedIPList(ocmConnection)
			Expect(err).To(BeNil())
		})
	})
})
