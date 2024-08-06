package config

import (
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift/backplane-cli/pkg/ocm"
	ocmMock "github.com/openshift/backplane-cli/pkg/ocm/mocks"
	"github.com/spf13/cobra"
)

var _ = Describe("get command", func() {

	var (
		mockCtrl         *gomock.Controller
		mockOcmInterface *ocmMock.MockOCMInterface
	)

	ocmEnv, _ := cmv1.NewEnvironment().BackplaneURL("https://backplane.example.com").Build()

	ocmEnvNil, _ := cmv1.NewEnvironment().BackplaneURL("").Build()

	cmd := &cobra.Command{Use: "get"}
	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockOcmInterface = ocmMock.NewMockOCMInterface(mockCtrl)
		ocm.DefaultOCMInterface = mockOcmInterface

	})
	Context("For backplane configuration", func() {
		It("Get backplane url return value", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			err := getConfig(cmd, []string{"url"})
			fmt.Println("Return value: ", err)
			Expect(err).To(BeNil())
		})

		It("Get backplane url return nil", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnvNil, nil).AnyTimes()
			err := getConfig(cmd, []string{"url"})
			fmt.Println("Return nil: ", err)
			Expect(err).To(BeNil())
		})

	})

})
