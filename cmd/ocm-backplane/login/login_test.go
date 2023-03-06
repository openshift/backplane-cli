package login

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/openshift/backplane-cli/pkg/client/mocks"
	"github.com/openshift/backplane-cli/pkg/utils"
	mocks2 "github.com/openshift/backplane-cli/pkg/utils/mocks"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

func MakeIoReader(s string) io.ReadCloser {
	r := io.NopCloser(strings.NewReader(s)) // r type is io.ReadCloser
	return r
}

var _ = Describe("Login command", func() {

	var (
		mockCtrl           *gomock.Controller
		mockClient         *mocks.MockClientInterface
		mockClientWithResp *mocks.MockClientWithResponsesInterface
		mockOcmInterface   *mocks2.MockOCMInterface
		mockClientUtil     *mocks2.MockClientUtils

		testClusterId string
		testToken     string
		trueClusterId string
		proxyUri      string

		fakeResp *http.Response
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
		testToken = "hello123"
		trueClusterId = "trueID123"
		proxyUri = "https://shard.apps"

		mockClientWithResp.EXPECT().LoginClusterWithResponse(gomock.Any(), gomock.Any()).Return(nil, nil).Times(0)

		fakeResp = &http.Response{
			Body:       MakeIoReader(`{"proxy_uri":"proxy", "statusCode":200, "message":"msg"}`),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeResp.Header.Add("Content-Type", "json")

	})

	AfterEach(func() {
		// Clear config file
		_ = clientcmd.ModifyConfig(clientcmd.NewDefaultPathOptions(), api.Config{}, true)
		clientcmd.UseModifyConfigLock = false
		args.backplaneURL = ""
		mockCtrl.Finish()
	})

	Context("runLogin function", func() {
		It("when running with a simple case should work as expected", func() {
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(proxyUri, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterId)).Return(fakeResp, nil)

			err := runLogin(nil, []string{testClusterId})

			Expect(err).To(BeNil())

			cfg, err := utils.ReadKubeconfigRaw()
			Expect(err).To(BeNil())
			Expect(cfg.CurrentContext).To(Equal("default/test123/anonymous"))
			Expect(len(cfg.Contexts)).To(Equal(1))
			Expect(cfg.Contexts["default/test123/anonymous"].Cluster).To(Equal(testClusterId))
			Expect(cfg.Contexts["default/test123/anonymous"].Namespace).To(Equal("default"))
		})

		It("Should fail when trying to find a non existent cluster", func() {
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()

			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return("", "", errors.New("err"))

			err := runLogin(nil, []string{testClusterId})

			Expect(err).ToNot(BeNil())
		})

		It("Should fail when failed to get OCM token", func() {
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(nil, errors.New("err")).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()

			err := runLogin(nil, []string{testClusterId})

			Expect(err).ToNot(BeNil())
		})

		It("should use the specified backplane url if passed", func() {
			args.backplaneURL = "https://sadge.app"
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken("https://sadge.app", testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterId)).Return(fakeResp, nil)

			err := runLogin(nil, []string{testClusterId})

			Expect(err).To(BeNil())

			cfg, err := utils.ReadKubeconfigRaw()
			Expect(err).To(BeNil())
			Expect(cfg.CurrentContext).To(Equal("default/test123/anonymous"))
			Expect(len(cfg.Contexts)).To(Equal(1))
			Expect(cfg.Contexts["default/test123/anonymous"].Cluster).To(Equal(testClusterId))
			Expect(cfg.Contexts["default/test123/anonymous"].Namespace).To(Equal("default"))
		})

		It("should fail if unable to create api client", func() {
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(proxyUri, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(proxyUri, testToken).Return(nil, errors.New("err"))

			err := runLogin(nil, []string{testClusterId})

			Expect(err).ToNot(BeNil())
		})

	})
})
