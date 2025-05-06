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
	. "github.com/onsi/ginkgo/v2"
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
		mockClient       *mocks.MockClientInterface

		testClusterID             string
		trueClusterID             string
		testToken                 string
		testRemediationName       string
		testRemediationInstanceID string
		fakeResp                  *http.Response
		bpConfigPath              string
		backplaneAPIURI           string
		ocmEnv                    *cmv1.Environment
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = mocks.NewMockClientInterface(mockCtrl)

		err := utils.CreateTempKubeConfig(nil)
		Expect(err).To(BeNil())

		mockOcmInterface = ocmMock.NewMockOCMInterface(mockCtrl)
		ocm.DefaultOCMInterface = mockOcmInterface

		mockClientUtil = backplaneapiMock.NewMockClientUtils(mockCtrl)
		backplaneapi.DefaultClientUtils = mockClientUtil

		mockClient.EXPECT().LoginCluster(gomock.Any(), gomock.Any()).Return(nil, nil).Times(0)

		_ = clientcmd.ModifyConfig(clientcmd.NewDefaultPathOptions(), api.Config{}, true)
		clientcmd.UseModifyConfigLock = false

		globalOpts.BackplaneURL = backplaneAPIURI

		ocmEnv, _ = cmv1.NewEnvironment().BackplaneURL("https://api.example.com").Build()

		fakeResp = &http.Response{
			Body:       MakeIoReader(`{"proxy_uri":"proxy", "statusCode":200, "message":"msg"}`),
			Header:     map[string][]string{},
			StatusCode: http.StatusOK,
		}
		fakeResp.Header.Add("Content-Type", "json")

		testClusterID = "test123"
		trueClusterID = "trueID123"
		backplaneAPIURI = "https://shard.apps.example.com/"
		testToken = "testToken"
		testRemediationName = "remediationName"
		testRemediationInstanceID = "remediationInstanceId"
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

			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().CreateRemediation(context.TODO(), trueClusterID, &BackplaneApi.CreateRemediationParams{RemediationName: testRemediationName}).Return(fakeResp, nil)

			err := runCreateRemediation([]string{testRemediationName}, testClusterID, backplaneAPIURI)

			Expect(err).To(BeNil())
		})

		It("Should use the custom URI if provided", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()

			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken("http://uri2.example.com", testToken).Return(mockClient, nil)
			mockClient.EXPECT().CreateRemediation(context.TODO(), trueClusterID, &BackplaneApi.CreateRemediationParams{RemediationName: testRemediationName}).Return(fakeResp, nil)

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

			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().CreateRemediation(context.TODO(), trueClusterID, &BackplaneApi.CreateRemediationParams{RemediationName: testRemediationName}).Return(fakeResp, nil)

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

			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(nil, errors.New("err"))

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

			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(mockClient, nil)
			mockClient.EXPECT().DeleteRemediation(context.TODO(), trueClusterID, &BackplaneApi.DeleteRemediationParams{RemediationInstanceId: testRemediationInstanceID}).Return(fakeResp, nil)

			err := runDeleteRemediation([]string{testRemediationInstanceID}, testClusterID, backplaneAPIURI)

			Expect(err).To(BeNil())
		})

		It("Should use the custom URI if provided", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetTargetCluster(testClusterID).Return(trueClusterID, testClusterID, nil).AnyTimes()
			mockOcmInterface.EXPECT().GetOCMAccessToken().Return(&testToken, nil)
			mockOcmInterface.EXPECT().IsClusterHibernating(gomock.Eq(trueClusterID)).Return(false, nil).AnyTimes()

			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken("https://uri2.example.com/", testToken).Return(mockClient, nil)
			mockClient.EXPECT().DeleteRemediation(context.TODO(), trueClusterID, &BackplaneApi.DeleteRemediationParams{RemediationInstanceId: testRemediationInstanceID}).Return(fakeResp, nil)

			err := runDeleteRemediation([]string{testRemediationInstanceID}, testClusterID, "https://uri2.example.com/")

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

			mockClientUtil.EXPECT().MakeRawBackplaneAPIClientWithAccessToken(backplaneAPIURI, testToken).Return(nil, errors.New("err"))

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
