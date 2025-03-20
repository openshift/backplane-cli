package ocm

import (
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	ocmMock "github.com/openshift/backplane-cli/pkg/ocm/mocks"
)

var _ = Describe("OCM Wrapper test", func() {

	var (
		mockCtrl *gomock.Controller

		mockOcmInterface *ocmMock.MockOCMInterface
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())

		mockOcmInterface = ocmMock.NewMockOCMInterface(mockCtrl)
		mockOcmInterface.EXPECT().SetupOCMConnection().Return(nil, nil)

		DefaultOCMInterface = mockOcmInterface
	})

	AfterEach(func() {

	})

	Context("Test OCM Wrapper", func() {

		It("Should return trusted IPList", func() {
			ip1 := cmv1.NewTrustedIp().ID("100.10.10.10")
			ip2 := cmv1.NewTrustedIp().ID("200.20.20.20")
			expectedIPList, _ := cmv1.NewTrustedIpList().Items(ip1, ip2).Build()
			mockOcmInterface.EXPECT().GetTrustedIPList().Return(expectedIPList, nil).AnyTimes()
			IPList, err := DefaultOCMInterface.GetTrustedIPList()
			Expect(err).To(BeNil())
			Expect(len(IPList.Items())).Should(Equal(2))
		})
		It("Should not return errors for empty trusted IPList", func() {
			mockOcmInterface.EXPECT().GetTrustedIPList().Return(nil, nil).AnyTimes()
			_, err := DefaultOCMInterface.GetTrustedIPList()
			Expect(err).To(BeNil())
		})
	})
})
