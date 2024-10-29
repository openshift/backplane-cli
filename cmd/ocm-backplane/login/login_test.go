package login

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/trivago/tgo/tcontainer"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/andygrunwald/go-jira"
	"github.com/openshift/backplane-cli/pkg/backplaneapi"
	backplaneapiMock "github.com/openshift/backplane-cli/pkg/backplaneapi/mocks"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/client/mocks"
	jiraClient "github.com/openshift/backplane-cli/pkg/jira"
	jiraMock "github.com/openshift/backplane-cli/pkg/jira/mocks"
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
		mockIssueService   *jiraMock.MockIssueServiceInterface

		testClusterID            string
		testToken                string
		trueClusterID            string
		managingClusterID        string
		backplaneAPIURI          string
		serviceClusterID         string
		serviceClusterName       string
		fakeResp                 *http.Response
		ocmEnv                   *cmv1.Environment
		kubeConfigPath           string
		mockCluster              *cmv1.Cluster
		backplaneConfiguration   config.BackplaneConfiguration
		falsePagerDutyAPITkn     string
		truePagerDutyAPITkn      string
		falsePagerDutyIncidentID string
		truePagerDutyIncidentID  string
		bpConfigPath             string
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
		falsePagerDutyAPITkn = "token123"
		// nolint:gosec truePagerDutyAPIToken refers to the Test API Token provided by https://developer.pagerduty.com/api-reference
		truePagerDutyAPITkn = "y_NbAkKc66ryYTWUXYEu"
		falsePagerDutyIncidentID = "incident123"
		truePagerDutyIncidentID = "Q0ZNH7TDQBOO54"

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

		mockCluster = &cmv1.Cluster{}

		backplaneConfiguration = config.BackplaneConfiguration{URL: backplaneAPIURI}

		loginType = LoginTypeClusterID
	})

	AfterEach(func() {
		globalOpts.Manager = false
		globalOpts.Service = false
		globalOpts.BackplaneURL = ""
		globalOpts.ProxyURL = ""
		os.Setenv("HTTPS_PROXY", "")
		os.Unsetenv("BACKPLANE_CONFIG")
		os.Remove(bpConfigPath)
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
			os.Setenv("HTTPS_PROXY", "https://squid.myproxy.com")
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

		It("when the namespace of the context is passed as an argument", func() {
			err := utils.CreateTempKubeConfig(nil)
			args.defaultNamespace = "default"
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
			Expect(cfg.Contexts["default/test123/anonymous"].Namespace).To(Equal(args.defaultNamespace))
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
			mockOcmInterface.EXPECT().GetClusterInfoByID(gomock.Any()).Return(mockCluster, nil).AnyTimes()
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
			loginType = LoginTypeExistingKubeConfig
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

		It("should fail to create PD API client and return HTTP status code 401 when unauthorized", func() {
			loginType = LoginTypePagerduty
			args.pd = truePagerDutyIncidentID

			err := utils.CreateTempKubeConfig(nil)
			Expect(err).To(BeNil())

			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()

			// Create a temporary JSON configuration file in the temp directory for testing purposes.
			tempDir := os.TempDir()
			bpConfigPath = filepath.Join(tempDir, "mock.json")
			tempFile, err := os.Create(bpConfigPath)
			Expect(err).To(BeNil())

			testData := config.BackplaneConfiguration{
				URL:              backplaneAPIURI,
				ProxyURL:         new(string),
				SessionDirectory: "",
				AssumeInitialArn: "",
				PagerDutyAPIKey:  falsePagerDutyAPITkn,
			}

			// Marshal the testData into JSON format and write it to tempFile.
			jsonData, err := json.Marshal(testData)
			Expect(err).To(BeNil())
			_, err = tempFile.Write(jsonData)
			Expect(err).To(BeNil())

			os.Setenv("BACKPLANE_CONFIG", bpConfigPath)

			err = runLogin(nil, nil)

			Expect(err.Error()).Should(ContainSubstring("status code 401"))
		})

		It("should return error when trying to login via PD but the PD API Key is not configured", func() {
			loginType = LoginTypePagerduty
			args.pd = truePagerDutyIncidentID

			err := utils.CreateTempKubeConfig(nil)
			Expect(err).To(BeNil())

			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()

			// Create a temporary JSON configuration file in the temp directory for testing purposes.
			tempDir := os.TempDir()
			bpConfigPath = filepath.Join(tempDir, "mock.json")
			tempFile, err := os.Create(bpConfigPath)
			Expect(err).To(BeNil())

			testData := config.BackplaneConfiguration{
				URL:             backplaneAPIURI,
				ProxyURL:        new(string),
				PagerDutyAPIKey: falsePagerDutyAPITkn,
			}

			// Marshal the testData into JSON format and write it to tempFile.
			pdTestData := testData
			pdTestData.PagerDutyAPIKey = ""
			jsonData, err := json.Marshal(pdTestData)
			Expect(err).To(BeNil())
			_, err = tempFile.Write(jsonData)
			Expect(err).To(BeNil())

			os.Setenv("BACKPLANE_CONFIG", bpConfigPath)

			err = runLogin(nil, nil)

			Expect(err.Error()).Should(ContainSubstring("please make sure the PD API Key is configured correctly"))
		})

		It("should fail to find a non existent PD Incident and return HTTP status code 404 when the requested resource is not found", func() {
			loginType = LoginTypePagerduty
			args.pd = falsePagerDutyIncidentID

			err := utils.CreateTempKubeConfig(nil)
			Expect(err).To(BeNil())

			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()

			// Create a temporary JSON configuration file in the temp directory for testing purposes.
			tempDir := os.TempDir()
			bpConfigPath = filepath.Join(tempDir, "mock.json")
			tempFile, err := os.Create(bpConfigPath)
			Expect(err).To(BeNil())

			testData := config.BackplaneConfiguration{
				URL:             backplaneAPIURI,
				ProxyURL:        new(string),
				PagerDutyAPIKey: truePagerDutyAPITkn,
			}

			// Marshal the testData into JSON format and write it to tempFile.
			jsonData, err := json.Marshal(testData)
			Expect(err).To(BeNil())
			_, err = tempFile.Write(jsonData)
			Expect(err).To(BeNil())

			os.Setenv("BACKPLANE_CONFIG", bpConfigPath)

			err = runLogin(nil, nil)

			Expect(err.Error()).Should(ContainSubstring("status code 404"))
		})

	})

	Context("check GetRestConfigAsUser", func() {
		It("check config creation with username and without elevationReasons", func() {
			mockOcmInterface.EXPECT().GetClusterInfoByID(testClusterID).Return(mockCluster, nil)
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(testClusterID)).Return(fakeResp, nil)

			username := "test-user"

			config, err := GetRestConfigAsUser(backplaneConfiguration, testClusterID, username, "")
			Expect(err).To(BeNil())
			Expect(config.Impersonate.UserName).To(Equal(username))
			Expect(len(config.Impersonate.Extra["reason"])).To(Equal(0))

		})

		It("check config creation with username and elevationReasons", func() {
			mockOcmInterface.EXPECT().GetClusterInfoByID(testClusterID).Return(mockCluster, nil)
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(testClusterID)).Return(fakeResp, nil)

			username := "test-user"
			elevationReasons := []string{"reason1", "reason2"}

			config, err := GetRestConfigAsUser(backplaneConfiguration, testClusterID, username, "", elevationReasons...)
			Expect(err).To(BeNil())
			Expect(config.Impersonate.UserName).To(Equal(username))
			Expect(config.Impersonate.Extra["reason"][0]).To(Equal(elevationReasons[0]))
			Expect(config.Impersonate.Extra["reason"][1]).To(Equal(elevationReasons[1]))

		})

	})

	Context("check JIRA OHSS login", func() {
		var (
			testOHSSID  string
			testIssue   jira.Issue
			issueFields *jira.IssueFields
		)
		BeforeEach(func() {
			mockIssueService = jiraMock.NewMockIssueServiceInterface(mockCtrl)
			ohssService = jiraClient.NewOHSSService(mockIssueService)
			testOHSSID = "OHSS-1000"
		})

		It("should login to ohss card cluster", func() {

			loginType = LoginTypeJira
			args.ohss = testOHSSID
			err := utils.CreateTempKubeConfig(nil)
			args.kubeConfigPath = ""
			Expect(err).To(BeNil())
			issueFields = &jira.IssueFields{
				Project:  jira.Project{Key: jiraClient.JiraOHSSProjectKey},
				Unknowns: tcontainer.MarshalMap{jiraClient.CustomFieldClusterID: testClusterID},
			}
			testIssue = jira.Issue{ID: testOHSSID, Fields: issueFields}
			globalOpts.ProxyURL = "https://squid.myproxy.com"
			mockIssueService.EXPECT().Get(testOHSSID, nil).Return(&testIssue, nil, nil).Times(1)
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockClientUtil.EXPECT().SetClientProxyURL(globalOpts.ProxyURL).Return(nil)
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(testClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(testClusterID)).Return(false, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Eq(testClusterID)).Return(fakeResp, nil)

			err = runLogin(nil, nil)

			Expect(err).To(BeNil())
		})

		It("should failed missing cluster id ohss cards", func() {

			loginType = LoginTypeJira
			args.ohss = testOHSSID

			issueFields = &jira.IssueFields{
				Project: jira.Project{Key: jiraClient.JiraOHSSProjectKey},
			}
			testIssue = jira.Issue{ID: testOHSSID, Fields: issueFields}
			mockIssueService.EXPECT().Get(testOHSSID, nil).Return(&testIssue, nil, nil).Times(1)
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()

			err := runLogin(nil, nil)

			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal("clusterID cannot be detected for JIRA issue:OHSS-1000"))
		})
	})
})
