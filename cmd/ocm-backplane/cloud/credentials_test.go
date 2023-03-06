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

		trueClusterId string
		proxyUri      string
		credentialAWS string
		credentialGcp string

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

		trueClusterId = "trueID123"
		proxyUri = "https://shard.apps"
		credentialAWS = "fake aws credential"
		credentialGcp = "fake gcp credential"

		mockClientWithResp.EXPECT().LoginClusterWithResponse(gomock.Any(), gomock.Any()).Return(nil, nil).Times(0)

		// Define fake AWS response
		fakeAWSResp = &http.Response{
			Body: MakeIoReader(
				fmt.Sprintf(`{"proxy_uri":"proxy", "statusCode":200, "message":"msg", "JSON200":"%s"}`, credentialAWS),
			),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeAWSResp.Header.Add("Content-Type", "json")

		// Define fake AWS response
		fakeGCloudResp = &http.Response{
			Body: MakeIoReader(
				fmt.Sprintf(`{"proxy_uri":"proxy", "statusCode":200, "message":"msg", "JSON200":"%s"}`, credentialGcp),
			),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeGCloudResp.Header.Add("Content-Type", "json")

		// Disabled log output
		log.SetOutput(io.Discard)
	})

	AfterEach(func() {
		// Clear config file
		_ = clientcmd.ModifyConfig(clientcmd.NewDefaultPathOptions(), api.Config{}, true)
		clientcmd.UseModifyConfigLock = false
		mockCtrl.Finish()
	})

	Context("test get Cloud Credential", func() {

		It("should return AWS cloud credential", func() {
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockClientUtil.EXPECT().GetBackplaneClient(proxyUri).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().GetCloudCredentials(gomock.Any(), trueClusterId).Return(fakeAWSResp, nil)

			crdentialResponse, err := getCloudCredential(proxyUri, trueClusterId)
			Expect(err).To(BeNil())

			Expect(crdentialResponse.JSON200).NotTo(BeNil())

		})

		/*It("should fail when AWS Unavailable", func() {
			fakeAWSResp.StatusCode = http.StatusInternalServerError
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockClientUtil.EXPECT().GetBackplaneClient(proxyUri).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().GetCloudCredentials(gomock.Any(), trueClusterId).Return(fakeAWSResp, nil)
			_, err := getCloudCredential(proxyUri, trueClusterId)
			Expect(err).NotTo(BeNil())

			Expect(err.Error()).Should(ContainSubstring("error from backplane: \n Status Code: 500\n"))

		})

		It("should fail when GCP Unavailable", func() {
			fakeGCloudResp.StatusCode = http.StatusInternalServerError
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockClientUtil.EXPECT().GetBackplaneClient(proxyUri).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().GetCloudCredentials(gomock.Any(), trueClusterId).Return(fakeGCloudResp, nil)
			_, err := getCloudCredential(proxyUri, trueClusterId)
			Expect(err).NotTo(BeNil())

			Expect(err.Error()).Should(ContainSubstring("error from backplane: \n Status Code: 500\n"))

		})

		It("should fail for unauthorized BP-API", func() {
			fakeAWSResp.StatusCode = http.StatusUnauthorized
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockClientUtil.EXPECT().GetBackplaneClient(proxyUri).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().GetCloudCredentials(gomock.Any(), trueClusterId).Return(fakeAWSResp, nil)
			_, err := getCloudCredential(proxyUri, trueClusterId)
			Expect(err).NotTo(BeNil())

			Expect(err.Error()).Should(ContainSubstring("error from backplane: \n Status Code: 401\n"))

		}) */
	})

})
