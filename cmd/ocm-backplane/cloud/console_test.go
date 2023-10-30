package cloud

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/openshift/backplane-cli/pkg/client/mocks"
	"github.com/openshift/backplane-cli/pkg/info"
	"github.com/openshift/backplane-cli/pkg/utils"
	mocks2 "github.com/openshift/backplane-cli/pkg/utils/mocks"
)

var _ = Describe("Cloud console command", func() {

	var (
		mockCtrl           *gomock.Controller
		mockClient         *mocks.MockClientInterface
		mockClientWithResp *mocks.MockClientWithResponsesInterface
		mockOcmInterface   *mocks2.MockOCMInterface
		mockClientUtil     *mocks2.MockClientUtils

		testClusterID    string
		trueClusterID    string
		proxyURI         string
		consoleAWSURL    string
		consoleGcloudURL string

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

		testClusterID = "test123"
		trueClusterID = "trueID123"
		proxyURI = "https://shard.apps"
		consoleAWSURL = "https://signin.aws.amazon.com/federation?Action=login"
		consoleGcloudURL = "https://cloud.google.com/"

		mockClientWithResp.EXPECT().LoginClusterWithResponse(gomock.Any(), gomock.Any()).Return(nil, nil).Times(0)

		// Define fake AWS response
		fakeAWSResp = &http.Response{
			Body: MakeIoReader(
				fmt.Sprintf(`{"proxy_uri":"proxy", "message":"msg", "ConsoleLink":"%s"}`, consoleAWSURL),
			),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeAWSResp.Header.Add("Content-Type", "json")

		// Define fake AWS response
		fakeGCloudResp = &http.Response{
			Body: MakeIoReader(
				fmt.Sprintf(`{"proxy_uri":"proxy", "message":"msg", "ConsoleLink":"%s"}`, consoleGcloudURL),
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
		os.Setenv(info.BackplaneURLEnvName, proxyURI)
	})

	AfterEach(func() {
		os.Setenv(info.BackplaneURLEnvName, "")
		mockCtrl.Finish()
	})

	Context("Execute cloud console command", func() {
		It("should return AWS cloud console", func() {
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().GetClusterInfoByID(trueClusterID).Return(nil, nil)
			mockClientUtil.EXPECT().GetBackplaneClient(proxyURI).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().GetCloudConsole(gomock.Any(), trueClusterID).Return(fakeAWSResp, nil)

			err := runConsole(nil, []string{testClusterID})

			Expect(err).To(BeNil())
		})

		It("should return GCP cloud console", func() {
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().GetClusterInfoByID(trueClusterID).Return(nil, nil)
			mockClientUtil.EXPECT().GetBackplaneClient(proxyURI).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().GetCloudConsole(gomock.Any(), trueClusterID).Return(fakeGCloudResp, nil)
			err := runConsole(nil, []string{testClusterID})

			Expect(err).To(BeNil())
		})

	})

	Context("get Cloud Console", func() {
		It("should return AWS cloud URL", func() {
			mockClient.EXPECT().GetCloudConsole(gomock.Any(), trueClusterID).Return(fakeAWSResp, nil)
			mockClientUtil.EXPECT().GetBackplaneClient(proxyURI).Return(mockClient, nil).AnyTimes()
			cloudResponse, err := getCloudConsole(proxyURI, trueClusterID)
			Expect(err).To(BeNil())

			Expect(cloudResponse.ConsoleLink).To(Equal(consoleAWSURL))

		})

		It("should return Gcloud cloud URL", func() {

			mockClientUtil.EXPECT().GetBackplaneClient(proxyURI).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().GetCloudConsole(gomock.Any(), trueClusterID).Return(fakeGCloudResp, nil)
			cloudResponse, err := getCloudConsole(proxyURI, trueClusterID)
			Expect(err).To(BeNil())

			Expect(cloudResponse.ConsoleLink).To(Equal(consoleGcloudURL))

		})

		It("should fail when AWS Unavailable", func() {
			fakeAWSResp.StatusCode = http.StatusInternalServerError

			mockClient.EXPECT().GetCloudConsole(gomock.Any(), trueClusterID).Return(fakeAWSResp, nil)
			mockClientUtil.EXPECT().GetBackplaneClient(proxyURI).Return(mockClient, nil).AnyTimes()
			_, err := getCloudConsole(proxyURI, trueClusterID)
			Expect(err).NotTo(BeNil())

			Expect(err.Error()).Should(ContainSubstring("error from backplane: \n Status Code: 500\n"))

		})

		It("should fail when GCP Unavailable", func() {
			fakeGCloudResp.StatusCode = http.StatusInternalServerError
			mockClientUtil.EXPECT().GetBackplaneClient(proxyURI).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().GetCloudConsole(gomock.Any(), trueClusterID).Return(fakeGCloudResp, nil)
			_, err := getCloudConsole(proxyURI, trueClusterID)
			Expect(err).NotTo(BeNil())

			Expect(err.Error()).Should(ContainSubstring("error from backplane: \n Status Code: 500\n"))

		})

		It("should fail for unauthorized BP-API", func() {
			fakeAWSResp.StatusCode = http.StatusUnauthorized
			mockClientUtil.EXPECT().GetBackplaneClient(proxyURI).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().GetCloudConsole(gomock.Any(), trueClusterID).Return(fakeAWSResp, nil)
			_, err := getCloudConsole(proxyURI, trueClusterID)
			Expect(err).NotTo(BeNil())

			Expect(err.Error()).Should(ContainSubstring("error from backplane: \n Status Code: 401\n"))

		})
	})
})
