package login

import (
	"errors"
	"io"
	"net/http"
	"os"
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

		testClusterId     string
		testToken         string
		trueClusterId     string
		managingClusterId string
		backplaneAPIUri   string

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
		managingClusterId = "managingID123"
		backplaneAPIUri = "https://shard.apps"

		mockClientWithResp.EXPECT().LoginClusterWithResponse(gomock.Any(), gomock.Any()).Return(nil, nil).Times(0)

		fakeResp = &http.Response{
			Body:       MakeIoReader(`{"proxy_uri":"proxy", "statusCode":200, "message":"msg"}`),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeResp.Header.Add("Content-Type", "json")
		// Clear config file
		_ = clientcmd.ModifyConfig(clientcmd.NewDefaultPathOptions(), api.Config{}, true)
		clientcmd.UseModifyConfigLock = false
	})

	AfterEach(func() {
		globalOpts.BackplaneURL = ""
		globalOpts.ProxyURL = ""
		os.Setenv("HTTPS_PROXY", "")
		mockCtrl.Finish()
	})

	Context("runLogin function", func() {
		It("when running with a simple case should work as expected", func() {
			err := utils.CreateTempKubeConfig(nil)
			Expect(err).To(BeNil())
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(backplaneAPIUri, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIUri, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterId)).Return(fakeResp, nil)

			err = runLogin(nil, []string{testClusterId})

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
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(backplaneAPIUri, nil).AnyTimes()

			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return("", "", errors.New("err"))

			err := runLogin(nil, []string{testClusterId})

			Expect(err).ToNot(BeNil())
		})

		It("Should fail when failed to get OCM token", func() {
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(nil, errors.New("err")).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(backplaneAPIUri, nil).AnyTimes()

			err := runLogin(nil, []string{testClusterId})

			Expect(err).ToNot(BeNil())
		})

		It("should use the specified backplane url if passed", func() {
			err := utils.CreateTempKubeConfig(nil)
			Expect(err).To(BeNil())
			globalOpts.BackplaneURL = "https://sadge.app"
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken("https://sadge.app", testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterId)).Return(fakeResp, nil)

			err = runLogin(nil, []string{testClusterId})

			Expect(err).To(BeNil())

			cfg, err := utils.ReadKubeconfigRaw()
			Expect(err).To(BeNil())
			Expect(cfg.CurrentContext).To(Equal("default/test123/anonymous"))
			Expect(len(cfg.Contexts)).To(Equal(1))
			Expect(cfg.Contexts["default/test123/anonymous"].Cluster).To(Equal(testClusterId))
			Expect(cfg.Contexts["default/test123/anonymous"].Namespace).To(Equal("default"))
		})

		It("should use the specified proxy url if passed", func() {
			err := utils.CreateTempKubeConfig(nil)
			Expect(err).To(BeNil())
			globalOpts.ProxyURL = "https://squid.myproxy.com"
			mockClientUtil.EXPECT().SetClientProxyUrl(globalOpts.ProxyURL).Return(nil)
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(backplaneAPIUri, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIUri, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterId)).Return(fakeResp, nil)

			err = runLogin(nil, []string{testClusterId})

			Expect(err).To(BeNil())

			cfg, err := utils.ReadKubeconfigRaw()

			Expect(err).To(BeNil())
			Expect(cfg.CurrentContext).To(Equal("default/test123/anonymous"))
			Expect(len(cfg.Contexts)).To(Equal(1))
			Expect(cfg.Contexts["default/test123/anonymous"].Cluster).To(Equal(testClusterId))
			Expect(cfg.Clusters[testClusterId].ProxyURL).To(Equal(globalOpts.ProxyURL))
			Expect(cfg.Contexts["default/test123/anonymous"].Namespace).To(Equal("default"))
		})

		It("should use the specified proxy if HTTPS_PROXY env var present", func() {
			err := utils.CreateTempKubeConfig(nil)
			Expect(err).To(BeNil())
			os.Setenv("HTTPS_PROXY", "https://squid2.myproxy.com")
			mockClientUtil.EXPECT().SetClientProxyUrl("https://squid2.myproxy.com").Return(nil)
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(backplaneAPIUri, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIUri, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterId)).Return(fakeResp, nil)

			err = runLogin(nil, []string{testClusterId})

			Expect(err).To(BeNil())

			cfg, err := utils.ReadKubeconfigRaw()

			Expect(err).To(BeNil())
			Expect(cfg.CurrentContext).To(Equal("default/test123/anonymous"))
			Expect(len(cfg.Contexts)).To(Equal(1))
			Expect(cfg.Contexts["default/test123/anonymous"].Cluster).To(Equal(testClusterId))
			Expect(cfg.Clusters[testClusterId].ProxyURL).To(Equal("https://squid2.myproxy.com"))
			Expect(cfg.Contexts["default/test123/anonymous"].Namespace).To(Equal("default"))
		})

		It("should fail if unable to create api client", func() {
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(backplaneAPIUri, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIUri, testToken).Return(nil, errors.New("err"))

			err := runLogin(nil, []string{testClusterId})

			Expect(err).ToNot(BeNil())
		})

		It("should return the managing cluster if one is requested", func() {
			args.manager = true
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterId).Return(trueClusterId, testClusterId, nil)
			mockOcmInterface.EXPECT().GetManagingCluster(trueClusterId).Return(managingClusterId, managingClusterId, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(managingClusterId)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockOcmInterface.EXPECT().GetBackplaneURL().Return(backplaneAPIUri, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIUri, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(managingClusterId)).Return(fakeResp, nil)

			err := runLogin(nil, []string{testClusterId})

			Expect(err).To(BeNil())
		})
	})
})
