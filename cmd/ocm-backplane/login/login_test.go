package login

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"

	"github.com/openshift/backplane-cli/pkg/backplaneapi"
	backplaneapiMock "github.com/openshift/backplane-cli/pkg/backplaneapi/mocks"
	"github.com/openshift/backplane-cli/pkg/client/mocks"
	"github.com/openshift/backplane-cli/pkg/login"
	"github.com/openshift/backplane-cli/pkg/ocm"
	ocmMock "github.com/openshift/backplane-cli/pkg/ocm/mocks"
	"github.com/openshift/backplane-cli/pkg/utils"
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
		mockOcmInterface   *ocmMock.MockOCMInterface
		mockClientUtil     *backplaneapiMock.MockClientUtils

		testClusterID      string
		testToken          string
		trueClusterID      string
		managingClusterID  string
		backplaneAPIURI    string
		serviceClusterID   string
		serviceClusterName string
		fakeResp           *http.Response
		ocmEnv             *cmv1.Environment
		kubeConfigPath     string
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = mocks.NewMockClientInterface(mockCtrl)
		mockClientWithResp = mocks.NewMockClientWithResponsesInterface(mockCtrl)

		mockOcmInterface = ocmMock.NewMockOCMInterface(mockCtrl)
		ocm.DefaultOCMInterface = mockOcmInterface

		mockClientUtil = backplaneapiMock.NewMockClientUtils(mockCtrl)
		backplaneapi.DefaultClientUtils = mockClientUtil

		testClusterID = "test123"
		testToken = "hello123"
		trueClusterID = "trueID123"
		managingClusterID = "managingID123"
		serviceClusterID = "hs-sc-123456"
		serviceClusterName = "hs-sc-654321"
		backplaneAPIURI = "https://shard.apps"
		kubeConfigPath = "filepath"

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

		globalOpts.BackplaneURL = backplaneAPIURI

		ocmEnv, _ = cmv1.NewEnvironment().BackplaneURL("https://dummy.api").Build()
	})

	AfterEach(func() {
		globalOpts.Manager = false
		globalOpts.Service = false
		globalOpts.BackplaneURL = ""
		globalOpts.ProxyURL = ""
		os.Setenv("HTTPS_PROXY", "")
		mockCtrl.Finish()
		utils.RemoveTempKubeConfig()
	})

	Context("check ocm token", func() {

		It("Should fail when failed to get OCM token", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(nil, errors.New("err")).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()

			err := runLogin(nil, []string{testClusterID})

			Expect(err).ToNot(BeNil())
		})

		It("should save ocm token to kube config", func() {
			err := utils.CreateTempKubeConfig(nil)
			Expect(err).To(BeNil())
			globalOpts.ProxyURL = "https://squid.myproxy.com"
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockClientUtil.EXPECT().SetClientProxyURL(globalOpts.ProxyURL).Return(nil)
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterID)).Return(fakeResp, nil)

			err = runLogin(nil, []string{testClusterID})

			Expect(err).To(BeNil())

			cfg, err := utils.ReadKubeconfigRaw()

			Expect(err).To(BeNil())
			Expect(cfg.AuthInfos["anonymous"].Token).To(Equal(testToken))
		})
	})

	Context("check BP-api and proxy connection", func() {

		It("should use the specified backplane url if passed", func() {
			err := utils.CreateTempKubeConfig(nil)
			Expect(err).To(BeNil())
			globalOpts.BackplaneURL = "https://sadge.app"
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken("https://sadge.app", testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterID)).Return(fakeResp, nil)

			err = runLogin(nil, []string{testClusterID})

			Expect(err).To(BeNil())

			cfg, err := utils.ReadKubeconfigRaw()
			Expect(err).To(BeNil())
			Expect(cfg.CurrentContext).To(Equal("default/test123/anonymous"))
			Expect(len(cfg.Contexts)).To(Equal(1))
			Expect(cfg.Contexts["default/test123/anonymous"].Cluster).To(Equal(testClusterID))
			Expect(cfg.Contexts["default/test123/anonymous"].Namespace).To(Equal("default"))
		})

		It("should use the specified proxy url if passed", func() {
			err := utils.CreateTempKubeConfig(nil)
			Expect(err).To(BeNil())
			globalOpts.ProxyURL = "https://squid.myproxy.com"
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockClientUtil.EXPECT().SetClientProxyURL(globalOpts.ProxyURL).Return(nil)
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterID)).Return(fakeResp, nil)

			err = runLogin(nil, []string{testClusterID})

			Expect(err).To(BeNil())

			cfg, err := utils.ReadKubeconfigRaw()

			Expect(err).To(BeNil())
			Expect(cfg.CurrentContext).To(Equal("default/test123/anonymous"))
			Expect(len(cfg.Contexts)).To(Equal(1))
			Expect(cfg.Contexts["default/test123/anonymous"].Cluster).To(Equal(testClusterID))
			Expect(cfg.Clusters[testClusterID].ProxyURL).To(Equal(globalOpts.ProxyURL))
			Expect(cfg.Contexts["default/test123/anonymous"].Namespace).To(Equal("default"))
		})

		It("should fail if unable to create api client", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(nil, errors.New("err"))

			err := runLogin(nil, []string{testClusterID})

			Expect(err).ToNot(BeNil())
		})

		It("should fail if ocm env backplane url is empty", func() {
			ocmEnv, _ = cmv1.NewEnvironment().Build()

			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			err := runLogin(nil, []string{testClusterID})

			Expect(err).ToNot(BeNil())
		})

	})

	Context("check cluster login", func() {
		BeforeEach(func() {
			globalOpts.Manager = false
			globalOpts.Service = false
		})
		It("when running with a simple case should work as expected", func() {
			err := utils.CreateTempKubeConfig(nil)
			Expect(err).To(BeNil())
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterID)).Return(fakeResp, nil)

			err = runLogin(nil, []string{testClusterID})

			Expect(err).To(BeNil())

			cfg, err := utils.ReadKubeconfigRaw()
			Expect(err).To(BeNil())
			Expect(cfg.CurrentContext).To(Equal("default/test123/anonymous"))
			Expect(len(cfg.Contexts)).To(Equal(1))
			Expect(cfg.Contexts["default/test123/anonymous"].Cluster).To(Equal(testClusterID))
			Expect(cfg.Contexts["default/test123/anonymous"].Namespace).To(Equal("default"))
		})

		It("Should fail when trying to find a non existent cluster", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()

			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return("", "", errors.New("err"))

			err := runLogin(nil, []string{testClusterID})

			Expect(err).ToNot(BeNil())
		})

		It("should return the managing cluster if one is requested", func() {
			globalOpts.Manager = true
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().GetManagingCluster(trueClusterID).Return(managingClusterID, managingClusterID, true, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(managingClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(managingClusterID)).Return(fakeResp, nil)

			err := runLogin(nil, []string{testClusterID})

			Expect(err).To(BeNil())
		})

		It("should failed if managing cluster not exist in same env", func() {
			globalOpts.Manager = true
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().GetManagingCluster(trueClusterID).Return(
				managingClusterID,
				managingClusterID,
				false,
				fmt.Errorf("failed to find management cluster for cluster %s in %s env", testClusterID, "http://test.env"),
			)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(managingClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil).AnyTimes()
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil).AnyTimes()
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(managingClusterID)).Return(fakeResp, nil).AnyTimes()

			err := runLogin(nil, []string{testClusterID})

			Expect(err).NotTo(BeNil())

			Expect(err.Error()).Should(ContainSubstring("failed to find management cluster for cluster test123 in http://test.env env"))
		})

		It("should return the service cluster if hosted cluster is given", func() {
			globalOpts.Service = true
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().GetServiceCluster(trueClusterID).Return(serviceClusterID, serviceClusterName, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(serviceClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(serviceClusterID)).Return(fakeResp, nil)

			err := runLogin(nil, []string{testClusterID})

			Expect(err).To(BeNil())
		})

		It("should login to current cluster if cluster id not provided", func() {
			err := utils.CreateTempKubeConfig(nil)
			Expect(err).To(BeNil())
			globalOpts.ProxyURL = "https://squid.myproxy.com"
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockClientUtil.EXPECT().SetClientProxyURL(globalOpts.ProxyURL).Return(nil)
			mockOcmInterface.EXPECT().GetTargetCluster("configcluster").Return(testClusterID, "dummy_cluster", nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(testClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(testClusterID)).Return(fakeResp, nil)

			err = runLogin(nil, nil)

			Expect(err).To(BeNil())

			cfg, err := utils.ReadKubeconfigRaw()
			Expect(err).To(BeNil())

			Expect(cfg.Contexts["default/test123/anonymous"].Cluster).To(Equal("dummy_cluster"))
			Expect(cfg.Contexts["default/test123/anonymous"].Namespace).To(Equal("default"))
		})

		It("should fail when a proxy or backplane url is unreachable", func() {

			err := utils.CreateTempKubeConfig(nil)
			Expect(err).To(BeNil())
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterID)).Return(fakeResp, errors.New("dial tcp: lookup yourproxy.com: no such host"))

			err = runLogin(nil, []string{testClusterID})

			Expect(err).NotTo(BeNil())

		})

		It("Check KUBECONFIG when logging into multiple clusters.", func() {
			globalOpts.Manager = false
			args.multiCluster = true
			err := utils.ModifyTempKubeConfigFileName(trueClusterID)
			Expect(err).To(BeNil())

			kubePath, err := os.MkdirTemp("", ".kube")
			Expect(err).To(BeNil())

			err = login.SetKubeConfigBasePath(kubePath)
			Expect(err).To(BeNil())

			_, err = login.CreateClusterKubeConfig(trueClusterID, utils.GetDefaultKubeConfig())
			Expect(err).To(BeNil())

			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(trueClusterID)).Return(fakeResp, nil)

			err = runLogin(nil, []string{testClusterID})

			Expect(os.Getenv("KUBECONFIG")).Should(ContainSubstring(trueClusterID))
			Expect(err).To(BeNil())

		})

		It("should fail if specify kubeconfigpath but not in multicluster mode", func() {
			globalOpts.Manager = false
			args.multiCluster = false
			args.kubeConfigPath = kubeConfigPath

			err := login.SetKubeConfigBasePath(args.kubeConfigPath)
			Expect(err).To(BeNil())

			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)

			err = runLogin(nil, []string{testClusterID})

			Expect(err.Error()).Should(ContainSubstring("can't save the kube config into a specific location if multi-cluster is not enabled"))

		})

	})
})
