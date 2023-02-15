package cloud

import (
	"fmt"
	"io"
	"net/http"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/openshift/backplane-cli/pkg/client/mocks"
	"github.com/openshift/backplane-cli/pkg/utils"
	mocks2 "github.com/openshift/backplane-cli/pkg/utils/mocks"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

var _ = Describe("Cloud console command", func() {

	var (
		mockCtrl           *gomock.Controller
		mockClient         *mocks.MockClientInterface
		mockClientWithResp *mocks.MockClientWithResponsesInterface
		mockOcmInterface   *mocks2.MockOCMInterface
		mockClientUtil     *mocks2.MockClientUtils

		testClusterId    string
		trueClusterId    string
		proxyUri         string
		consoleAWSUrl    string
		consoleGcloudUrl string

		fakeAWSResp    *http.Response
		fakeGCloudResp *http.Response
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = mocks.NewMockClientInterface(mockCtrl)
		mockClientWithResp = mocks.NewMockClientWithResponsesInterface(mockCtrl)

		mockOcmInterface = mocks2.NewMockOCMInterface(mockCtrl)
		utils.DefaultOCMInterface = mockOcmInterface

		mockClientUtil = mocks2.NewMockClientUtils(mockCtrl)
		utils.DefaultClientUtils = mockClientUtil

		testClusterId = "test123"
		trueClusterId = "trueID123"
		proxyUri = "https://shard.apps"
		consoleAWSUrl = "https://signin.aws.amazon.com/federation?Action=login"
		consoleGcloudUrl = "https://cloud.google.com/"

		mockClientWithResp.EXPECT().LoginClusterWithResponse(gomock.Any(), gomock.Any()).Return(nil, nil).Times(0)

		// Define fake AWS response
		fakeAWSResp = &http.Response{
			Body: MakeIoReader(
				fmt.Sprintf(`{"proxy_uri":"proxy", "statusCode":200, "message":"msg", "ConsoleLink":"%s"}`, consoleAWSUrl),
			),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeAWSResp.Header.Add("Content-Type", "json")

		// Define fake AWS response
		fakeGCloudResp = &http.Response{
			Body: MakeIoReader(
				fmt.Sprintf(`{"proxy_uri":"proxy", "statusCode":200, "message":"msg", "ConsoleLink":"%s"}`, consoleGcloudUrl),
			),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeGCloudResp.Header.Add("Content-Type", "json")

		// Clear config file
		_ = clientcmd.ModifyConfig(clientcmd.NewDefaultPathOptions(), api.Config{}, true)
		clientcmd.UseModifyConfigLock = false

		// Disabled log output
		log.SetOutput(io.Discard)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Context("Execute cloud console command", func() {
		It("should return AWS cloud console", func() {
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockClientUtil.EXPECT().GetBackplaneClient(proxyUri).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().GetCloudConsole(gomock.Any(), trueClusterId).Return(fakeAWSResp, nil)

			err := runConsole(nil, []string{testClusterId})

			Expect(err).To(BeNil())
		})

		It("should return GCP cloud console", func() {
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockClientUtil.EXPECT().GetBackplaneClient(proxyUri).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().GetCloudConsole(gomock.Any(), trueClusterId).Return(fakeGCloudResp, nil)
			err := runConsole(nil, []string{testClusterId})

			Expect(err).To(BeNil())
		})

	})

	Context("get Cloud Console", func() {
		It("should return AWS cloud URL", func() {
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockClient.EXPECT().GetCloudConsole(gomock.Any(), trueClusterId).Return(fakeAWSResp, nil)
			mockClientUtil.EXPECT().GetBackplaneClient(proxyUri).Return(mockClient, nil).AnyTimes()
			cloudResponse, err := getCloudConsole(proxyUri, trueClusterId)
			Expect(err).To(BeNil())

			Expect(cloudResponse.ConsoleLink).To(Equal(consoleAWSUrl))

		})

		It("should return Gcloud cloud URL", func() {
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockClientUtil.EXPECT().GetBackplaneClient(proxyUri).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().GetCloudConsole(gomock.Any(), trueClusterId).Return(fakeGCloudResp, nil)
			cloudResponse, err := getCloudConsole(proxyUri, trueClusterId)
			Expect(err).To(BeNil())

			Expect(cloudResponse.ConsoleLink).To(Equal(consoleGcloudUrl))

		})

		It("should fail when AWS Unavailable", func() {
			fakeAWSResp.StatusCode = http.StatusInternalServerError
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockClient.EXPECT().GetCloudConsole(gomock.Any(), trueClusterId).Return(fakeAWSResp, nil)
			mockClientUtil.EXPECT().GetBackplaneClient(proxyUri).Return(mockClient, nil).AnyTimes()
			_, err := getCloudConsole(proxyUri, trueClusterId)
			Expect(err).NotTo(BeNil())

			Expect(err.Error()).Should(ContainSubstring("error from backplane: \n Status Code: 500\n"))

		})

		It("should fail when GCP Unavailable", func() {
			fakeGCloudResp.StatusCode = http.StatusInternalServerError
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockClientUtil.EXPECT().GetBackplaneClient(proxyUri).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().GetCloudConsole(gomock.Any(), trueClusterId).Return(fakeGCloudResp, nil)
			_, err := getCloudConsole(proxyUri, trueClusterId)
			Expect(err).NotTo(BeNil())

			Expect(err.Error()).Should(ContainSubstring("error from backplane: \n Status Code: 500\n"))

		})

		It("should fail for unauthorized BP-API", func() {
			fakeAWSResp.StatusCode = http.StatusUnauthorized
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockClientUtil.EXPECT().GetBackplaneClient(proxyUri).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().GetCloudConsole(gomock.Any(), trueClusterId).Return(fakeAWSResp, nil)
			_, err := getCloudConsole(proxyUri, trueClusterId)
			Expect(err).NotTo(BeNil())

			Expect(err.Error()).Should(ContainSubstring("error from backplane: \n Status Code: 401\n"))

		})
	})
})
