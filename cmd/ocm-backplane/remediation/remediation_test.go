package remediation

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	BackplaneApi "github.com/openshift/backplane-api/pkg/client"
	"github.com/openshift/backplane-cli/pkg/backplaneapi"
	backplaneapiMock "github.com/openshift/backplane-cli/pkg/backplaneapi/mocks"
	"github.com/openshift/backplane-cli/pkg/client/mocks"
	"github.com/openshift/backplane-cli/pkg/ocm"
	ocmMock "github.com/openshift/backplane-cli/pkg/ocm/mocks"
	"github.com/openshift/backplane-cli/pkg/utils"
)

func MakeIoReader(s string) io.ReadCloser {
	r := io.NopCloser(strings.NewReader(s)) // r type is io.ReadCloser
	return r
}

var _ = Describe("New Remediation command", func() {
	var (
		mockCtrl         *gomock.Controller
		mockOcmInterface *ocmMock.MockOCMInterface
		mockClientUtil   *backplaneapiMock.MockClientUtils
		//mockClient         *mocks.MockClientInterface
		mockClientWithResp *mocks.MockClientWithResponsesInterface
		//mockCluster        *cmv1.Cluster

		testClusterID             string
		trueClusterID             string
		testToken                 string
		testRemediationName       string
		testProxyURI              string
		fakeResp                  *http.Response
		bpConfigPath              string
		backplaneAPIURI           string
		ocmEnv                    *cmv1.Environment
		createRemediationResponse *BackplaneApi.CreateRemediationResponse
		deleteRemediationResponse *BackplaneApi.DeleteRemediationResponse
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClientWithResp = mocks.NewMockClientWithResponsesInterface(mockCtrl)
		//ockClient = mocks.NewMockClientInterface(mockCtrl)

		mockOcmInterface = ocmMock.NewMockOCMInterface(mockCtrl)
		ocm.DefaultOCMInterface = mockOcmInterface

		mockClientUtil = backplaneapiMock.NewMockClientUtils(mockCtrl)
		backplaneapi.DefaultClientUtils = mockClientUtil

		mockClientWithResp.EXPECT().LoginClusterWithResponse(gomock.Any(), gomock.Any()).Return(nil, nil).Times(0)

		_ = clientcmd.ModifyConfig(clientcmd.NewDefaultPathOptions(), api.Config{}, true)
		clientcmd.UseModifyConfigLock = false

		globalOpts.BackplaneURL = backplaneAPIURI

		ocmEnv, _ = cmv1.NewEnvironment().BackplaneURL("https://api.example.com").Build()

		//mockCluster = &cmv1.Cluster{}

		//backplaneConfiguration = config.BackplaneConfiguration{URL: backplaneAPIURI}

		fakeResp = &http.Response{
			Body:       MakeIoReader(`{"proxy_uri":"proxy", "statusCode":200, "message":"msg"}`),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeResp.Header.Add("Content-Type", "json")

		testProxyURI = "http://proxy.example.com/"

		createRemediationResponse = &BackplaneApi.CreateRemediationResponse{
			HTTPResponse: fakeResp,
			Body:         []byte(""),
			JSON200:      &BackplaneApi.LoginResponse{ProxyUri: &testProxyURI},
		}

		deleteRemediationResponse = &BackplaneApi.DeleteRemediationResponse{
			HTTPResponse: fakeResp,
			Body:         []byte(""),
		}

		testClusterID = "test123"
		trueClusterID = "trueID123"
		backplaneAPIURI = "https://shard.apps.example.com/"
		testToken = "testToken"
		testRemediationName = "remediationName"

	})

	AfterEach(func() {
		globalOpts.Manager = false
		globalOpts.Service = false
		globalOpts.BackplaneURL = ""
		globalOpts.ProxyURL = ""
		os.Setenv("HTTPS_PROXY", "")
		os.Setenv("HTTP_PROXY", "")
		os.Unsetenv("BACKPLANE_CONFIG")
		os.Remove(bpConfigPath)
		mockCtrl.Finish()
		utils.RemoveTempKubeConfig()
	})
	Context("create Remediation", func() {
		It("Should run the query without returning an error", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()

			mockClientUtil.EXPECT().MakeBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClientWithResp, nil)
			mockClientWithResp.EXPECT().CreateRemediationWithResponse(context.TODO(), trueClusterID, &BackplaneApi.CreateRemediationParams{Remediation: testRemediationName}).Return(createRemediationResponse, nil)

			err := runCreateRemediation([]string{testRemediationName}, testClusterID, backplaneAPIURI)

			Expect(err).To(BeNil())
		})

		It("Should use the custom URI if provided", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()

			mockClientUtil.EXPECT().MakeBackplaneAPIClientWithAccessToken("http://uri2.example.com", testToken).Return(mockClientWithResp, nil)
			mockClientWithResp.EXPECT().CreateRemediationWithResponse(context.TODO(), trueClusterID, &BackplaneApi.CreateRemediationParams{Remediation: testRemediationName}).Return(createRemediationResponse, nil)

			err := runCreateRemediation([]string{testRemediationName}, testClusterID, "http://uri2.example.com")

			Expect(err).To(BeNil())
		})

		It("Should use the Proxy URL set in global opts", func() {
			globalOpts.ProxyURL = "https://squid.example.com"
			os.Setenv("HTTPS_PROXY", "https://squid.example.com")

			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockClientUtil.EXPECT().SetClientProxyURL(globalOpts.ProxyURL).Return(nil)
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()

			mockClientUtil.EXPECT().MakeBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClientWithResp, nil)
			mockClientWithResp.EXPECT().CreateRemediationWithResponse(context.TODO(), trueClusterID, &BackplaneApi.CreateRemediationParams{Remediation: testRemediationName}).Return(createRemediationResponse, nil)

			err := runCreateRemediation([]string{testRemediationName}, testClusterID, backplaneAPIURI)

			Expect(err).To(BeNil())
		})

		It("Should fail when failed to get OCM token", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(nil, errors.New("err")).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()

			err := runCreateRemediation([]string{testRemediationName}, testClusterID, backplaneAPIURI)

			Expect(err).ToNot(BeNil())
		})

		It("Should fail if unable to create API client", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()

			mockClientUtil.EXPECT().MakeBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(nil, errors.New("err"))

			err := runCreateRemediation([]string{testRemediationName}, testClusterID, backplaneAPIURI)

			Expect(err).ToNot(BeNil())
		})

		It("Should fail if backplaneURL is empty", func() {
			ocmEnv, _ = cmv1.NewEnvironment().Build()

			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()

			err := runCreateRemediation([]string{testRemediationName}, testClusterID, backplaneAPIURI)

			Expect(err).ToNot(BeNil())
		})
	})

	Context("delete remediation", func() {
		It("Should run the query without returning an error", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()

			mockClientUtil.EXPECT().MakeBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClientWithResp, nil)
			mockClientWithResp.EXPECT().DeleteRemediationWithResponse(context.TODO(), trueClusterID, &BackplaneApi.DeleteRemediationParams{Remediation: &testRemediationName}).Return(deleteRemediationResponse, nil)

			err := runDeleteRemediation([]string{testRemediationName}, testClusterID, backplaneAPIURI)

			Expect(err).To(BeNil())
		})

		It("Should use the custom URI if provided", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()

			mockClientUtil.EXPECT().MakeBackplaneAPIClientWithAccessToken("https://uri2.example.com/", testToken).Return(mockClientWithResp, nil)
			mockClientWithResp.EXPECT().DeleteRemediationWithResponse(context.TODO(), trueClusterID, &BackplaneApi.DeleteRemediationParams{Remediation: &testRemediationName}).Return(deleteRemediationResponse, nil)

			err := runDeleteRemediation([]string{testRemediationName}, testClusterID, "https://uri2.example.com/")

			Expect(err).To(BeNil())
		})

		It("Should fail when failed to get OCM token", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil)
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(nil, errors.New("err")).AnyTimes()
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()

			err := runDeleteRemediation([]string{testRemediationName}, testClusterID, backplaneAPIURI)

			Expect(err).ToNot(BeNil())
		})

		It("Should fail if unable to create API client", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()

			mockClientUtil.EXPECT().MakeBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(nil, errors.New("err"))

			err := runDeleteRemediation([]string{testRemediationName}, testClusterID, backplaneAPIURI)

			Expect(err).ToNot(BeNil())
		})

		It("Should fail if backplaneURL is empty", func() {
			ocmEnv, _ = cmv1.NewEnvironment().Build()

			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()

			err := runDeleteRemediation([]string{testRemediationName}, testClusterID, backplaneAPIURI)

			Expect(err).ToNot(BeNil())
		})
	})

})

func TestIt(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Login Test Suite")
}
